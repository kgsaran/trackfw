# Modelo de ameaça — escrita atômica do CLI Python no Windows (ML-0A)

> Autor: hades-tf | Data: 2026-09-01 | Roadmap:
> `docs/roadmaps/wip/ROADMAP-2026-09-01-escrita-atomica-do-cli-python-funciona-no-windows.md` | REQ:
> `docs/req/REQ-2026-09-01-os-fchmod-e-unix-only-e-derruba-as-tres-escritas-atomicas-do-cli-python-no-windows.md`

**Escopo desta nota:** só análise. Nenhuma linha de `pypi/`, `internal/`, `npm/`, `scripts/` foi
tocada. Todas as PoCs abaixo rodaram em cópias/diretórios de `/tmp`, nunca no home real nem no
repositório.

## Resumo executivo

0. **`os.fchmod` só é load-bearing em 2 dos 7 pontos de chamada — o resto pede `0o600`, que
   `tempfile.mkstemp()` já entrega por padrão.** Ver seção 0 — isto muda onde o gate da Wave 2
   precisa mirar e onde o teste de controle do AC3(b) precisa realmente provar algo.
1. **A janela de TOCTOU do `os.fchmod(fd)` de hoje é fechada corretamente** — não há regressão a
   proteger contra, porque a primitiva atual nunca teve a janela.
2. **A troca ingênua para `os.chmod(path)` reabre a janela, e ela é comprovadamente explorável — não
   teórica —, condicionada a um pré-requisito concreto que hoje NÃO é garantido pelo código:** o
   diretório-pai `.trackfw` frequentemente **não** fica em `0o700` como o código presume, porque
   `mode=` de `os.makedirs`/`Path.mkdir` **é ignorado quando o diretório já existe** e **não se propaga
   a diretórios-pai intermediários** — comprovado ao vivo (seção 1).
3. **Achei uma segunda janela de TOCTOU, pré-existente, independente da decisão sobre `fchmod` vs.
   `chmod`, presente nos três arquivos HOJE, em produção:** o `os.replace(temporary, filename)` final
   opera sobre o **caminho** do temporário, não sobre o descritor. Um ataque de troca de symlink no
   mesmo intervalo que ameaçaria o `chmod(path)` também sequestra o `os.replace` — **mesmo com
   `os.fchmod(fd)`**. Isto está **fora do escopo desta REQ** (o `os.replace` não muda com a troca de
   primitiva de chmod), mas precisa virar item de acompanhamento — ver seção 6.
4. **Enumeração:** `os.fchmod` é a única API POSIX-only usada como decisão de segurança/permissão em
   `pypi/`. Não achei `os.fchown`, `os.chown`, `os.geteuid`, `os.getuid`, `os.umask`, `os.symlink`
   sem guarda, `os.link`, `os.mkfifo`, nem uso de `stat.S_IS*` como decisão que quebre em Windows —
   os únicos `stat.S_IS*` existentes (`manager.py`, `update.py`) são chamadas portáveis, cobertas por
   nota de vault preexistente sobre junction (ver seção 4). **Desta vez os três batem — não há
   subestimativa.**
5. **Triplicação:** dos "três" `_atomic_write`, só **dois são cópias físicas independentes sem
   relação declarada** (`identity/__init__.py` e `integrations/manager.py`); o terceiro
   (`thirdparty/quarantine.py`) já é compartilhado **por import** dentro do próprio pacote
   `thirdparty` (`references.py` e `provenance.py` importam a função de `quarantine.py`, não a
   duplicam). A assimetria real: **só a cópia de `quarantine.py` tem doc-comment explicando por que
   não importa de `integrations/manager.py`; a de `identity/__init__.py` não tem nenhuma
   justificativa registrada.** Vereditos na seção 5.

---

## 0 — `os.fchmod` é load-bearing em quais dos 7 pontos de chamada?

`tempfile.mkstemp()` cria o arquivo já em `0o600` (dono apenas), sempre, independente de umask —
comportamento do próprio módulo `tempfile`, não do nosso código. Levantei todo valor literal de
`mode` que chega a `_atomic_write`/`fchmod`:

| Site | `mode` | `fchmod` muda algo? |
|---|---|---|
| `identity/__init__.py:134` (`save`) | `0o600` | **Não** — já é o modo padrão do `mkstemp` |
| `integrations/manager.py:139` (`_write_manifest`) | `0o600` | **Não** |
| `thirdparty/quarantine.py:110` | `0o600` | **Não** |
| `thirdparty/references.py:79` | `0o600` | **Não** |
| `thirdparty/provenance.py:117` | `0o600` | **Não** |
| `integrations/manager.py:343` (restore de snapshot no rollback) | `mode` (variável, herdada do snapshot original — captura o modo real do arquivo antes de sobrescrever) | **Sim, potencialmente** — se o artefato restaurado for um dos que usam `0o644` (linha abaixo), o restore herda o mesmo efeito observável |
| `integrations/manager.py:358` → `_plan_artifact_write` (linha 583): `pending = (destination, plan["content"], 0o644)` | **`0o644`** | **Sim — é o ponto onde `fchmod` produz efeito observável de forma mais direta**: alarga de `0o600` (padrão do `mkstemp`) para `0o644` (leitura para grupo/outros) |

**Isso muda o que o AC2/AC3(b) precisam provar.** Nos 5 sites de `0o600`, um teste de controle que só
verifica "`os.fchmod` foi chamado com `(descriptor, mode)`" **passaria mesmo numa implementação que
descartasse a chamada silenciosamente** — o modo final seria `0o600` de qualquer forma, por causa do
`mkstemp`, não por causa do `fchmod`. Os dois sites onde `fchmod` **produz um efeito observável em
POSIX** são a instalação/atualização de artefato de integração (`:358`, mode `0o644`) e o restore de
snapshot no rollback (`:343`, quando o snapshot restaurado é um desses artefatos `0o644`), ambos em
`integrations/manager.py`. **O teste de controle do AC3(b) só prova algo real se for escrito contra
UM DESSES dois sites** (asserir `os.stat(destino).st_mode & 0o777 == 0o644` depois de um
`install`/`update` real, não contra `identity.save()` ou `quarantine`, onde `0o600` é vácuo por
construção).

**Segunda razão, além do alargamento, para `fchmod` ser load-bearing especificamente aqui —
importante para quem for desenhar a Wave 1:** `os.open`/`tempfile.mkstemp` (modo aplicado na
criação) **é mascarado por umask**; `os.fchmod`/`os.chmod` **não é**. Um implementador que "resolvesse"
o Windows aplicando `mode=0o644` já na criação do temporário (em vez de chamar `fchmod` depois)
passaria um teste `st_mode & 0o777 == 0o644` num runner com `umask 022` (022 não mascara nenhum bit
de `0o644`) e **falharia silenciosamente sob `umask 077`**, produzindo `0o600` em vez de `0o644` — um
vetor de falsificação que não está na tabela da seção 3.2 e deveria ser adicionado lá pela Wave 1/2.

Isso também torna a PoC da seção 1.3 mais forte: em vez de demonstrar corrupção 0644→0600 (um
*aperto*, cenário defensivo), a PoC 2 abaixo demonstra o caso real do código — **alargamento**
0600→0644 de um arquivo alheio, com o valor literal `0o644` que `manager.py` de fato usa.

## 1 — A janela é alcançável neste código, ou é teórica?

### 1.1 O que `os.fchmod(fd)` garante hoje

`tempfile.mkstemp()` já cria o arquivo temporário com `0o600` (dono apenas), independente de umask —
isso é comportamento documentado do módulo `tempfile`, não do nosso código. O `os.fchmod(descriptor,
mode)` atua sobre o **descritor já aberto**: não há resolução de caminho entre a criação e a
aplicação do modo. **Não existe janela para esta primitiva, hoje, em nenhum dos três arquivos.**

### 1.2 Onde a suposição do roadmap quebra: o diretório-pai não é confiavelmente `0o700`

O roadmap presume "o diretório-pai já é criado com `mode=0o700`". Testei isso ao vivo e a suposição
**é falsa na prática**, por duas razões independentes, ambas comprovadas por execução (não leitura):

**(a) `mode=` é ignorado quando o diretório já existe.** Documentado no próprio `os.makedirs`, mas o
efeito prático não estava medido. Simulação: instalar os hooks de guarda globais primeiro (como
`update_harness.py` faz, com `Path(path).parent.mkdir(parents=True, exist_ok=True)` — **sem
`mode=`**), depois chamar `identity.save()`:

```
$ umask
022
$ python3 -c "... cria ~/.trackfw/scripts sem mode, depois roda identity.save() ..."
0o755 .../.trackfw
0o755 .../.trackfw/scripts
0o755 .../.trackfw          <- ainda 755 DEPOIS do identity.save() com mode=0o700
0o600 .../.trackfw/identity.json
```

`identity.save()` passou `mode=0o700` para `os.makedirs(directory, exist_ok=True, mode=0o700)`, mas
como `.trackfw` **já existia** (criado antes pelo instalador de scripts, sem `mode=`), o pedido foi
silenciosamente ignorado. O arquivo final (`identity.json`) fica corretamente em `0o600` — isso é o
próprio `_atomic_write` que garante, não o diretório —, mas o **diretório que o hospeda** fica em
`0o755`, não `0o700`.

**(b) `mode=` de `Path.mkdir(parents=True, mode=X)` só se aplica à FOLHA, nunca aos pais
intermediários criados no mesmo `mkdir`.** Comprovado com um projeto **totalmente vazio**, sem
`.trackfw` prévio, chamando só `quarantine._atomic_write`:

```
0o755 <projeto>/.trackfw                        <- pai intermediário: default do umask
0o700 <projeto>/.trackfw/thirdparty-quarantine   <- folha: mode=0o700 aplicado corretamente
```

Ou seja: mesmo no melhor caso (projeto novo, ninguém tocou `.trackfw` antes), o `.trackfw` do projeto
já nasce em `0o755`, e só o subdiretório-folha (`thirdparty-quarantine`) fica corretamente restrito.
Para `quarantine.py` isso é **suficiente** — os arquivos de quarentena vivem na folha, que está
correta. Para `identity.py` e para `integrations/manager.py` (quando escrevem em `root/.trackfw/`
diretamente, sem subdiretório-folha própria) **não é suficiente**, porque o diretório que hospeda o
arquivo sensível É o `.trackfw`, que não é a folha em nenhum dos casos onde outra coisa já criou
`.trackfw` primeiro.

### 1.3 Isso é explorável, ou só "feio"?

`0o755` dá leitura+travessia a outros usuários locais, mas **não escrita** — sob umask padrão (022),
ninguém além do dono pode inserir/remover/substituir entradas no diretório, então o ataque de troca de
symlink (que exige escrever no diretório) **não é alcançável sob condições padrão**. A vulnerabilidade
fica **latente**, não ativa, na configuração mais comum.

Ela se torna ativa quando:
- `umask=0` — nesse caso `.trackfw` nasceria `0o777`, mundialmente gravável. **Não afirmo que isso
  seja o padrão de containers/CI** — verifiquei os workflows deste repositório
  (`grep -rn "umask" .github/workflows/`) e **nenhum** define umask explicitamente, então os runners
  do GitHub Actions usados por este projeto herdam o padrão do runner (tipicamente `0022` em
  Ubuntu/macOS), **não** `0`. `umask=0` fica como cenário de ambiente relaxado manualmente (alguns
  containers base de terceiros, alguns scripts de provisionamento não auditados por este projeto),
  não como algo que eu tenha medido acontecer aqui;
- um administrador ou ferramenta externa relaxou a permissão do diretório manualmente (`chmod 777
  ~/.trackfw` por engano, script de setup de terceiro, etc.);
- multiusuário no mesmo host com esse `HOME` compartilhado por engano (cenário incomum, mas não nulo
  para diretórios de projeto compartilhados via NFS/samba com múltiplos UIDs mapeados).

**PoC 1** (permissão arbitrária, cenário geral): diretório deliberadamente tornado gravável (`0o777`,
simulando os cenários listados acima) e um atacante correndo em thread separada entre `mkstemp()` e a
chamada de permissão:

```python
import os, stat, tempfile, threading, time

def show(label, p):
    st = os.lstat(p)
    kind = 'SYMLINK->' + os.readlink(p) if stat.S_ISLNK(st.st_mode) else 'file'
    print(f"[{label}] {p} mode={oct(stat.S_IMODE(st.st_mode))} {kind}")

def naive_chmod_write(filename, mode):
    directory = os.path.dirname(filename)
    descriptor, temporary = tempfile.mkstemp(prefix=".trackfw-tmp-", dir=directory)
    os.close(descriptor)
    def attacker():
        time.sleep(0.05)
        os.unlink(temporary)
        os.symlink(victim, temporary)
    t = threading.Thread(target=attacker); t.start(); time.sleep(0.1)
    os.chmod(temporary, mode)              # <-- A CHAMADA VULNERAVEL: segue symlink
    t.join()
    os.replace(temporary, filename)

def fchmod_write(filename, data, mode):
    directory = os.path.dirname(filename)
    descriptor, temporary = tempfile.mkstemp(prefix=".trackfw-tmp-", dir=directory)
    def attacker():
        time.sleep(0.05)
        os.unlink(temporary)
        os.symlink(victim, temporary)
    t = threading.Thread(target=attacker); t.start(); time.sleep(0.1)
    os.fchmod(descriptor, mode)            # opera no fd -- a troca de caminho e irrelevante aqui
    with os.fdopen(descriptor, "wb") as s: s.write(data); s.flush(); os.fsync(s.fileno())
    t.join()
    os.replace(temporary, filename)        # mas o replace ainda usa o caminho -- ver secao 6

d = tempfile.mkdtemp(prefix="hades-toctou-")
os.chmod(d, 0o777)
victim = os.path.join(d, "victim-owned-by-someone-else")
open(victim, "w").write("do not touch\n"); os.chmod(victim, 0o644)

naive_chmod_write(os.path.join(d, "identity.json"), 0o600)
show("victim apos naive_chmod_write", victim)   # mode=0o600 <- CORROMPIDO (era 0o644)
show("identity.json", os.path.join(d, "identity.json"))  # SYMLINK -> victim

os.chmod(victim, 0o644)  # reset
fchmod_write(os.path.join(d, "identity2.json"), b'{"legit":true}', 0o600)
show("victim apos fchmod_write", victim)        # mode=0o644 <- INTOCADO
show("identity2.json", os.path.join(d, "identity2.json"))  # ainda SYMLINK -> victim (sec. 6)
```

Saída real, obtida rodando o script acima:

```
[victim apos naive_chmod_write] .../victim-owned-by-someone-else mode=0o600 file
[identity.json] .../identity.json mode=0o755 SYMLINK->.../victim-owned-by-someone-else
[victim apos fchmod_write] .../victim-owned-by-someone-else mode=0o644 file
[identity2.json] .../identity2.json mode=0o755 SYMLINK->.../victim-owned-by-someone-else
```

Com `os.chmod(path)` (fix ingênuo), a permissão do arquivo-alvo é corrompida (`0o644` → `0o600`, o
atacante escolhe o valor) **e** `identity.json` passa a ser um symlink para o arquivo alheio — a
próxima `load()` lê o conteúdo escolhido pelo atacante. Com `os.fchmod(fd)` (código atual), a
permissão do alvo fica intocada — mas `identity2.json` **ainda** vira o symlink, porque o
`os.replace` final continua operando sobre o caminho (ver seção 6, janela separada e pré-existente).

**PoC 2** (o caso real do código — alargamento `0o600`→`0o644`, valor literal de
`integrations/manager.py:_plan_artifact_write`, seção 0): mesma estrutura de ataque, `mode=0o644`
contra uma vítima privada `0o600`:

```python
import os, stat, tempfile, threading, time

d = tempfile.mkdtemp(prefix="hades-toctou-widen-")
os.chmod(d, 0o777)
victim = os.path.join(d, "private-key-like-file")
open(victim, "w").write("segredo\n"); os.chmod(victim, 0o600)  # privado, como o mkstemp deixaria

target = os.path.join(d, "artifact-under-install")
descriptor, temporary = tempfile.mkstemp(prefix=".trackfw-tmp-", dir=d)
os.close(descriptor)

def attacker():
    time.sleep(0.05)
    os.unlink(temporary)
    os.symlink(victim, temporary)

t = threading.Thread(target=attacker); t.start(); time.sleep(0.1)
os.chmod(temporary, 0o644)   # <-- exatamente o mode literal de manager.py:_plan_artifact_write
t.join()
os.replace(temporary, target)

st = os.stat(victim)
print(oct(st.st_mode & 0o777))
```

Saída real: `victim` era `0o600` (privado, o mesmo estado que `mkstemp` deixaria por padrão) e passa
a `0o644` (leitura para grupo e outros) — **alargamento de permissão de um arquivo alheio, produzido
pelo fallback ingênuo, no valor de `mode` que o código realmente usa**, não num valor hipotético.

### Veredito 1

A janela do `os.chmod(path)` ingênuo **é real, não teórica**, mas seu alcance depende de uma
condição que hoje **não é garantida pelo próprio código** (diretório-pai restritivo) — nem sob umask
padrão ela chega a ser explorável por outro usuário, mas o código está, por acidente, um umask
incomum de distância de ficar explorável, e isso nunca foi verificado antes de hoje. **Recomendação
para a Wave 1: o fallback correto é `getattr(os, "fchmod", None)` com `chmod(path)` só quando
`fchmod` não existe** (Windows, onde o TOCTOU do NTFS/ACL não é o modelo de ameaça do POSIX) — igual
ao AC2 já pede — **e, como achado de acompanhamento não bloqueante para esta REQ**, recomendo abrir
REQ separada para consertar o `mode=` do `.trackfw` (pai) via `os.chmod(directory, 0o700)` explícito
após `makedirs`, não confiar no argumento `mode=` para diretórios pré-existentes ou intermediários.

---

## 2 — Enumeração de APIs POSIX-only em `pypi/`

Varredura por `os.fchmod`, `os.fchown`, `os.chown`, `os.geteuid`, `os.getuid`, `os.getegid`,
`os.umask`, `os.symlink` sem guarda, `os.link`, `os.mkfifo`, `stat.S_IS*` como decisão de segurança,
em todo `pypi/trackfw` **incluindo `tests/`** (a varredura inicial excluía `tests/`; refeita sem essa
exclusão após revisão — o histórico desta sessão registra duas subestimativas anteriores por excluir
população real, e um `os.fchmod` do lado de teste conta para a linha de base de 145 falhas do CI de
Windows citada pelo roadmap tanto quanto um de produção). Também ampliada para `st_ino`, `st_dev`,
`O_NOFOLLOW`, `os.lchmod`, `shutil.chown` — comparação de identidade de inode é o parente
silencioso deste bug (não confiável em algumas configurações de Windows) e `os.lchmod` é a
"correção óbvia" seguinte que alguém pode tentar na Wave 1 sem perceber que **também não existe no
Linux** (só BSD/macOS) — trocaria um crash por outro, num SO diferente:

```
$ grep -rn "fchmod\|st_ino\|st_dev\|O_NOFOLLOW\|lchmod\|shutil\.chown" pypi/
pypi/trackfw/identity/__init__.py:97:        os.fchmod(descriptor, mode)
pypi/trackfw/integrations/manager.py:120:            os.fchmod(descriptor, mode)
pypi/trackfw/thirdparty/quarantine.py:42:        os.fchmod(descriptor, mode)
```

Zero ocorrências em `tests/` (nenhum teste hoje faz mock/monkeypatch de `os.fchmod` — a Wave 1
precisará introduzir o primeiro), zero `st_ino`/`st_dev`/`O_NOFOLLOW`/`lchmod`/`shutil.chown` em todo
o `pypi/`. **Confirma que não há um quarto local escondido em teste, nem uma dependência oculta de
identidade de inode.** (Achado à parte, não relacionado à contagem: existe uma quarta cópia de
`os.fchmod` em `pypi/build/lib/trackfw/...` — artefato de build local, coberto por
`.gitignore:11:pypi/build/`, confirmado não rastreado pelo git; não é um quarto local de produção.)

| API | Ocorrências | Veredito |
|---|---|---|
| `os.fchmod` | 3 — `identity/__init__.py:97`, `integrations/manager.py:120`, `thirdparty/quarantine.py:42` | Confirma a lista da REQ, exaustiva |
| `os.fchown` / `os.chown` | 0 | — |
| `os.geteuid` / `os.getuid` / `os.getegid` | 0 | — |
| `os.umask` | 0 | — |
| `os.symlink` | 0 chamadas de escrita (o único `os.symlink` do repo está na PoC desta nota, fora de `pypi/`) | — |
| `os.link` / `os.mkfifo` | 0 | — |
| `stat.S_ISLNK`/`S_ISREG`/`S_ISDIR` | `integrations/manager.py:90,296,594`, `commands/update.py:668,680` | Portáveis (o módulo `stat` funciona em Windows); o risco real associado — `lstat` não distingue junction do NTFS de diretório comum — já está registrado em `vault/notes/lstat-nao-ve-junction-e-guarda-de-folha-nao-olha-ancestral-2026-08-31.md`, não é regressão desta REQ |
| `os.chmod(path, ...)` (não relacionado a `_atomic_write`) | 7 ocorrências, todas em scripts de shell/hook (`update.py:276`, `discover.py:413`, `init_gen.py:538,1847,1852,1868,1885,1907,1928`) | `os.chmod` **existe** no Windows (efeito limitado, mas não levanta `AttributeError`) — não é o defeito desta REQ. Fora de escopo. |

**Veredito 2:** `os.fchmod` é a única API verdadeiramente POSIX-only usada como decisão de
permissão/segurança em `pypi/`. Nesta rodada a contagem do usuário (3) **bate** com a varredura —
diferente das duas ocasiões anteriores citadas na REQ.

---

## 3 — Falsificação nas duas direções

### 3.1 Direta (a ausência de `os.fchmod` não derruba a escrita)

Cenário: `getattr(os, "fchmod", None) is None` (simulando Windows). As três escritas devem concluir
sem `AttributeError`, com o arquivo final tendo o mesmo conteúdo e (best-effort) o `mode` pedido via
`os.chmod(path, mode)` como fallback.

### 3.2 Simétrica — a que mais importa, porque falha em silêncio

O risco nomeado na REQ é um fallback que **dispara em POSIX por engano**, sem derrubar nenhum teste
existente. Os vetores concretos, e como cada um se detecta:

| Vetor de regressão silenciosa | Por que passaria despercebido | Teste que o pega |
|---|---|---|
| `getattr(os, "fchmod", os.chmod)` (default errado: usa `os.chmod` como *default value*, não só quando ausente) | Em POSIX, `os.fchmod` existe, então `getattr` retorna a função certa — **mas se o default for mal escrito como `lambda fd, m: os.chmod(caminho_capturado_errado, m)`**, o teste "arquivo tem o mode certo" ainda passaria, só a *rota* estaria errada | **Não basta checar o resultado.** É preciso instrumentar a chamada: `monkeypatch.setattr(os, "fchmod", Mock(wraps=os.fchmod))` e afirmar `mock.called is True` e `mock.call_args == (descriptor, mode)` — controle positivo do AC3(b) |
| `try: os.fchmod(...) except AttributeError: os.chmod(path, mode)` capturando um `AttributeError` **não relacionado** (ex.: typo em outra variável dentro do mesmo `try`) | Em ambos os SOs o teste "escreveu e tem o mode certo" passaria, mascarando um bug não relacionado como "estamos no Windows" | **Não envolver `fchmod` num `try/except AttributeError` amplo.** Checar `hasattr(os, "fchmod")` (ou `getattr(os, "fchmod", None) is not None`) **uma vez**, fora do bloco que pode lançar por outros motivos — decisão de capacidade, não tratamento de exceção |
| Mock global de teste que sombreia `os.fchmod` sem restaurar (`os.fchmod = lambda *a: None` direto, em vez de `monkeypatch`) | Se um teste de OUTRO módulo faz isso e não usa fixture com cleanup automático, testes que rodam depois, na mesma sessão pytest, herdam o mock — os testes desta REQ passariam mesmo com o fallback "sempre ativo" | Buscar, no `pypi/trackfw/tests/`, todo `os.fchmod = ` direto (atribuição crua) fora de `monkeypatch`/`unittest.mock.patch` como contexto — nenhum encontrado nesta varredura, mas o gate da Wave 2 deve proibir esse padrão daqui pra frente |
| Import condicional errado (`if sys.platform == "win32": from os import chmod as fchmod` colado fora do módulo de teste, em código de produção) | Se a condição estiver invertida (`!=` em vez de `==`), POSIX passaria a rodar o fallback **sempre**, e nenhum teste que só roda em Windows perceberia — precisa rodar em CI POSIX real | AC7 já exige isso: a suíte completa (camada 1) precisa incluir o job POSIX além do Windows; o gate de paridade sozinho (camada 2) não pega |

**Veredito 3:** a defesa contra a falha simétrica não é "testar que o Windows funciona" — é
**testar, no job POSIX do CI, que `os.fchmod` foi efetivamente chamado com os argumentos certos**
(controle positivo), mais **proibir por revisão/gate** qualquer `except AttributeError` amplo ao
redor da chamada. Recomendo que a AC5/gate da Wave 2 grep por `except AttributeError` nas
proximidades imediatas de `_atomic_write` e reprove se encontrar (falsificável, sem regex frágil
demais — a ancoragem exata fica para a Wave 1/2, que sabe a forma final do código).

**Correção sobre ONDE o controle positivo prova algo (decorre da seção 0):** o `mock.call_args ==
(descriptor, mode)` só é um controle não-vácuo no site de `integrations/manager.py` que escreve
artefato com `mode=0o644` (seção 0). Nos outros cinco sites (`0o600`), asserir "`fchmod` foi chamado"
prova que a *rota de código* foi exercida, mas **não prova que o `fchmod` mudou nada** — uma
implementação que chamasse `os.fchmod(descriptor, mode)` e o resultado fosse idêntico ao que
`mkstemp` já entrega passaria nesse controle mesmo se a chamada fosse um no-op por acidente de outra
natureza. O teste que efetivamente falsifica "o fallback enfraqueceu a garantia" é: **depois de um
`install`/`update` real com artefato novo, `os.stat(destino).st_mode & 0o777 == 0o644`** — esse é o
único ponto onde um `0o600` residual (fchmod não rodou/foi sombreado) e um `0o644` correto (fchmod
rodou) são **observáveis por resultado**, sem precisar instrumentar `os.fchmod`. Recomendo a Wave 1
escrever o teste de controle do AC3(b) contra esse site, não contra `identity.save()`.

---

## 4 — Extrair vs. gatear a triplicação

### 4.1 O que é realmente triplicado

`references.py` e `provenance.py` **não duplicam** `_atomic_write` — importam de `quarantine.py`
(`from .quarantine import _atomic_write`). A REQ e o roadmap descrevem "três implementações
replicadas", mas na prática são **duas cópias físicas independentes sem nenhuma relação declarada**
(`identity/__init__.py`, `integrations/manager.py`) mais **uma terceira que já resolveu a duplicação
dentro do próprio pacote `thirdparty`** (`quarantine.py`, reusada por import por dois outros módulos
do mesmo pacote).

### 4.2 A assimetria que a REQ não nomeou

`quarantine.py:34-37` documenta, com justificativa explícita, por que **não importa** de
`integrations/manager.py` — manter `thirdparty` independente de `trackfw.integrations`. Essa é uma
decisão registrada, e vale o que protege: o pacote `thirdparty` (que processa conteúdo baixado de
terceiros, superfície de maior desconfiança do projeto) não herda uma dependência de import-time de
um pacote de propósito geral.

`identity/__init__.py` **não tem nenhum comentário equivalente**. Não há decisão registrada
explicando por que `identity` não importa de `integrations/manager.py`, nem por que
`integrations/manager.py` não importa de `identity`. Isso não é, por si, um defeito — mas significa
que **a REQ está pedindo para "decidir sobre a triplicação" um caso em que 1 das 2 relações
relevantes nunca foi decidida por ninguém**, só aconteceu.

### 4.3 Veredito

**Não extrair.** A independência do `thirdparty` em relação a `integrations` está documentada e
protege a superfície certa (conteúdo de terceiro não deveria puxar código de propósito geral por
import-time só para persistir um registro). Estender esse raciocínio a `identity`: identidade de
agente é configuração de baixa sensibilidade de fluxo (não processa conteúdo de terceiro), então o
argumento de isolamento é mais fraco ali — mas ainda assim, unificar as **três** definições num
helper compartilhado criaria uma dependência nova de `identity` → algum módulo comum, e de
`thirdparty` → o mesmo módulo comum, quebrando exatamente a garantia que `quarantine.py:34-37`
registrou.

**Contra-argumento óbvio, e por que não muda o veredito:** um módulo-folha novo (ex.
`trackfw/_atomicio.py`), sem nenhuma dependência de `trackfw.integrations` nem de `trackfw.thirdparty`,
não violaria literalmente `quarantine.py:34-37` — a ressalva ali é especificamente sobre não importar
`trackfw.integrations`, não sobre "não ter nenhum helper compartilhado". Mesmo assim prefiro gatear,
por dois motivos que a extração não resolve: (a) o próprio comentário de `quarantine.py` já pesou essa
opção implicitamente e optou por replicar em vez de introduzir QUALQUER acoplamento novo — mesmo um
módulo-folha é uma dependência nova a auditar toda vez que `thirdparty` processa conteúdo não
confiável, e o ônus da prova de que ela é segura cabe a quem propuser, não a esta nota; (b) a extração
resolve a divergência **estrutural** (as três chamando funções diferentes por acidente), mas não
resolve a ausência de justificativa da cópia de `identity/__init__.py` (item 2 abaixo) — esse gap é
de documentação/decisão, não de arquitetura, e persistiria mesmo com um módulo-folha compartilhado se
ninguém registrar por que `identity` tampouco importa dele. **O remédio correto é gate, não
extração**, e o gate precisa cobrir os dois riscos:

1. **Divergência de comportamento entre as três cópias** (ex.: alguém corrige duas de três com o
   fallback do AC2 e esquece a terceira) — gate estrutural: as três definições de `_atomic_write`
   devem ter a mesma sequência de chamadas relevantes (`mkstemp` → guarda de capacidade de `fchmod` →
   `fchmod` condicional → `fdopen`/`write`/`fsync` → `os.replace`), verificável por grep ancorado
   (mesmo padrão já usado noutros gates deste projeto, ex.
   `scripts/check-ref-separator-portability.sh`), não por importação — a Wave 2 é quem desenha o
   texto exato do gate, esta nota só fixa o requisito.
2. **A cópia de `identity/__init__.py` não tem doc-comment.** Recomendo que a Wave 1 adicione um
   comentário simétrico ao de `quarantine.py`, nomeando as três localizações e a razão para não
   importar — isso não é escopo desta nota de ameaça implementar, mas é um critério de aceite que
   deveria ser adicionado ao roadmap da Wave 1 (achado de acompanhamento, não bloqueante para o
   ML-0A).

---

## 5 — Residual declarado

Mesmo assumindo que a Wave 1 implemente o fallback condicional (`getattr(os, "fchmod", None)`)
corretamente nos três arquivos, com controle positivo no POSIX e gate anti-divergência entre as três
cópias, os seguintes riscos **permanecem, deliberadamente, fora do que esta REQ resolve**:

1. **A janela do `os.replace(temporary, filename)` final (seção 6)** — pré-existente, independente da
   escolha de chmod, presente hoje nos três arquivos, em produção. Não é regressão desta REQ e não
   deve bloqueá-la, mas precisa de REQ de acompanhamento própria — o remédio (verificar
   `O_NOFOLLOW`/`os.stat` do caminho logo antes do `replace`, ou mover para um padrão baseado em
   `os.rename` com verificação de inode) tem seu próprio espaço de decisão que não cabe aqui.
2. **`.trackfw` (o diretório-pai, não a folha) não fica confiavelmente em `0o700`** quando outro
   código já o criou primeiro sem `mode=` — comprovado ao vivo na seção 1.2. Fora do escopo desta
   REQ (que trata do mecanismo de `chmod` do *arquivo*, não do *diretório*), mas é a peça que torna a
   seção 1.3 mais que puramente acadêmica em ambientes de umask não-padrão. Recomendo REQ separada:
   aplicar `os.chmod(directory, 0o700)` explicitamente após `makedirs`/`mkdir`, independentemente de o
   diretório já existir ou de quantos níveis foram criados no mesmo `mkdir(parents=True)`.
3. **Ambiente Windows não tem o mesmo modelo de ameaça do POSIX** — o fallback via `os.chmod(path,
   mode)` no Windows não fecha TOCTOU (o NTFS/ACL do Windows não tem o conceito de permissão Unix
   nativamente; `os.chmod` no Windows só alterna o bit somente-leitura). Isso é aceitável **porque o
   modelo de ameaça desta REQ é "não regredir o POSIX"**, não "alcançar paridade de garantia com o
   POSIX no Windows" — mas deve ficar dito explicitamente no AC6/contrato de `docs/cli-parity.md`,
   para que ninguém leia "escrita atômica funciona no Windows" como "tem a mesma garantia TOCTOU do
   POSIX".
4. **Multiusuário com `HOME` compartilhado ou umask não-padrão** não é hoje um modelo de ameaça
   nomeado em nenhum ADR/REQ deste projeto. Esta nota assume implicitamente o modelo usual de CLI de
   desenvolvedor (single-user workstation) como baseline "normal", e trata umask=0/diretório
   compartilhado como caso degradado, não como ataque no modelo de ameaça padrão do projeto. Se esse
   modelo de ameaça for elevado a "sempre considerar" no futuro (ex.: trackfw rodando em ambiente de
   CI compartilhado com outros processos não confiáveis), esta nota precisa ser revisitada.
5. **O CLI Node já tem a mesma classe de TOCTOU, hoje, em produção, independente desta REQ** —
   `npm/src/thirdparty/quarantine.js:29` e `npm/src/integrations/manager.js:95,97` usam
   `chmodSync(path, ...)` (caminho) em vez de `fchmodSync(fd, ...)` (descritor, disponível no Node).
   Fora do escopo desta REQ (que é Python-only), mas condiciona a redação do contrato do AC6 — ver
   seção 7.

---

## 6 — Superfície não prevista pelo prompt: o `os.replace` final também tem caminho, não descritor

Acho que vale destacar de novo, fora da estrutura das 5 perguntas, porque é o achado mais fácil de
perder: **a escolha entre `os.fchmod(fd)` e `os.chmod(path)` só protege o passo de PERMISSÃO.** O
passo seguinte, `os.replace(temporary, filename)`, sempre opera sobre o **caminho** do arquivo
temporário — em nenhuma das duas variantes isso é resolvido por descritor. Minha PoC da seção 1.3
mostra isso explicitamente: mesmo com `os.fchmod(fd)` (código atual, correto), se o atacante consegue
trocar `temporary` por um symlink **depois do `fchmod` mas antes do `replace`**, o `os.replace` ainda
move o **symlink**, não o conteúdo, para o destino final.

Isso não é um defeito introduzido pela Wave 1 desta REQ — é **pré-existente, nos três arquivos, hoje**
— e não deveria ser resolvido como parte deste ML-0A (a REQ delimitou "não alterar as permissões
pedidas, só o mecanismo de aplicá-las" — este achado é sobre um mecanismo diferente, o `replace`, não
o `chmod`). Registro aqui para virar REQ de acompanhamento; não bloqueia a Wave 1/2 desta REQ porque
está condicionado ao mesmo pré-requisito da seção 1 (diretório gravável por outro usuário), que hoje
não é o caso sob umask padrão.

---

## 7 — Achado fora do escopo de `pypi/`: o CLI Node já tem, hoje, o TOCTOU que esta REQ existe para
não introduzir no Python

A REQ nomeia a tabela dos três runtimes (`docs/req/.../REQ-2026-09-01-...md`, linha ~39) como prova
de que só o Python tem o defeito de crash, e cita `fs.chmodSync(tmp, mode)` do Node como "funciona no
Windows" — verdade para o crash, mas a tabela não avalia TOCTOU nos outros dois runtimes, e o meu
escopo (`pypi/`) também não cobriria isso a menos que eu olhasse de propósito. Olhei, porque a AC6
desta REQ escreve um contrato **cross-CLI** em `docs/cli-parity.md` ("escrita atômica preserva a
garantia de descritor onde a plataforma oferece") — esse contrato só faz sentido se eu souber se os
outros dois runtimes já o cumprem.

```
$ grep -rn "chmodSync\|mkstemp\|mkdtempSync\|renameSync\|openSync" npm/src/ | grep -v test
```

Resultado relevante, por arquivo:

| Arquivo | Padrão | TOCTOU? |
|---|---|---|
| `npm/src/identity/config.js:77-84` | `fs.openSync(temporaryName, 'w', mode)` — o modo é aplicado **na criação do arquivo**, via `open(2)`, não por um `chmod` posterior — depois só `fs.renameSync` | **Seguro por construção**, equivalente em garantia ao `os.fchmod(fd)` do Python: não há chamada de permissão resolvendo caminho |
| `npm/src/thirdparty/quarantine.js:28-30` | `fs.writeFileSync(tmp, data, { mode })` (aplica `mode` na criação, via `open()` interno do Node — **mascarado por umask**, igual a `os.open`/`mkstemp` no Python) **seguido de `fs.chmodSync(tmp, mode)`** antes do `renameSync` | **TOCTOU real, hoje, em produção — e o `chmodSync` não é supérfluo, é quem entrega o `mode` pedido sob umask não-padrão.** `chmod`/`fchmod` **não é** mascarado por umask, diferente do `mode` passado a `open()`; sob `umask 077` (ou qualquer umask que mascare bits de `mode`), o `writeFileSync({mode})` sozinho **não** produziria o modo pedido — o `chmodSync` é quem corrige isso, exatamente como o `os.fchmod` do Python é a peça que aplica o `mode` pedido além do que `mkstemp` cria por padrão. O problema não é a chamada existir — é que ela resolve **caminho**, não descritor: `fs.fchmodSync(fd, mode)` (existe no Node, confirmado: `typeof require('fs').fchmodSync === 'function'`) entregaria a mesma garantia sem a janela. Mesma classe de ataque da seção 1.3/PoC 1 desta nota, sem precisar de nenhuma mudança de plataforma para existir |
| `npm/src/integrations/manager.js:94-97` | `fs.writeFileSync(tmp, content, { mode })` **seguido de `fs.chmodSync(tmp, mode)` (pré-rename, mesma função de defeat-de-umask acima) E de um SEGUNDO `fs.chmodSync(file, mode)` pós-`renameSync`** | **Duas janelas TOCTOU**, pior que `quarantine.js`: a primeira tem a mesma função legítima (garantir `mode` sob umask não-padrão) e o mesmo defeito (resolve caminho); a segunda chmod'a o **caminho final**, já publicado, depois do rename — uma janela adicional que nem o Python nem `identity.js`/`quarantine.js` do próprio Node têm, porque nenhum outro site chama chmod depois do rename. `manager.js:362` usa o mesmo literal `mode: 0o644` do Python (`_plan_artifact_write`) para instalação de artefato — mesmo site, mesmo valor, mesma classe de exposição |

**Isto não é regressão desta REQ, nem foi introduzido por ela — é o estado atual do `npm/`,
independente de qualquer mudança no Python.** Mas é diretamente relevante para o AC6: se a Wave 2
escrever o contrato *"escrita atômica preserva a garantia de descritor onde a plataforma oferece"* em
`docs/cli-parity.md` sem qualificar, o contrato publicado seria **falso para `manager.js` e
`quarantine.js` do Node hoje** — um gate de paridade que tentasse verificar esse contrato nos três
runtimes reprovaria o Node, não o Python. As opções, que cabem ao arquiteto/Wave 2 decidir, não a
mim: (a) nomear isto explicitamente como exceção documentada de paridade em `docs/cli-parity.md`,
até que uma REQ separada troque os `chmodSync(path, ...)` do Node por `fchmodSync(fd, ...)` — **não
por remover as chamadas**, que continuariam necessárias sob umask não-padrão, só trocar a primitiva
de caminho por descritor, o mesmo remédio que esta REQ já aplica ao Python; ou (b) expandir o escopo
desta REQ para cobrir os três runtimes — **não recomendo (b)**: a REQ já delimita "não unificar
`_atomic_write` com Go/Node — são runtimes diferentes com primitivas diferentes", e misturar um fix
de TOCTOU do Node dentro de uma REQ sobre crash do Python no Windows dilui os dois. **Recomendo REQ
de acompanhamento dedicada ao Node**, e que a AC6 desta REQ registre a exceção em vez de escrever um
contrato que o próprio repositório já viola.
