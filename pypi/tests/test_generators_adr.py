"""
Testes unitários para pypi/trackfw/generators/adr.py e pypi/trackfw/commands/adr.py
Formato canônico Go/Node — REQ-2026-07-27-convergencia-templates-python.
Casos de --scope global/project — REQ-2026-08-08-adr-new-com-escopo-global-scope-global-
escrevendo-em-trackfw-adr.
"""

import argparse
import io
import os
import re
import shutil
import sys
import tempfile
import unittest
from contextlib import contextmanager, redirect_stdout
from datetime import date

_HERE = os.path.dirname(os.path.abspath(__file__))
_PYPI = os.path.dirname(_HERE)
if _PYPI not in sys.path:
    sys.path.insert(0, _PYPI)

from trackfw.generators.adr import slugify, generate_adr, global_adr_dir, list_adrs
from trackfw import config as trackfw_config
from trackfw.commands import adr as adr_cmd



@contextmanager
def _chdir(path):
    """`os.chdir(path)` que SEMPRE volta ao cwd anterior ao sair do bloco.

    ML-4B. No Windows o cwd do processo mantem um HANDLE aberto sobre o diretorio, e
    `os.rmdir` sobre ele falha com `PermissionError: [WinError 32] The process cannot access
    the file because it is being used by another process` -- foi assim que 4 testes desta
    classe morreram no CI de Windows, na LIMPEZA do `TemporaryDirectory`, nao no corpo. Em
    POSIX o diretorio e removivel enquanto e cwd, entao o defeito e invisivel localmente.

    O `tearDown` nao resolve: ele roda DEPOIS que o `with tempfile.TemporaryDirectory()` ja
    tentou apagar o diretorio.

    ORDEM E CARGA UTIL: este gerenciador tem de ser o ULTIMO da cadeia `with`, porque os
    gerenciadores saem em ordem inversa -- assim o cwd e restaurado ANTES da limpeza do
    tmpdir. Colocado primeiro, o bug volta e continua passando em POSIX.
    """
    previous = os.getcwd()
    os.chdir(path)
    try:
        yield path
    finally:
        os.chdir(previous)


class TestSlugify(unittest.TestCase):

    def test_slugify_acento(self):
        """Acentos devem ser removidos e espaços viram hifens."""
        result = slugify('Minha Decisão Técnica')
        self.assertEqual(result, 'minha-decisao-tecnica')

    def test_slugify_simples(self):
        result = slugify('Authentication Strategy')
        self.assertEqual(result, 'authentication-strategy')

    def test_slugify_caracteres_especiais(self):
        """Caracteres não-alfanuméricos exceto hífen devem ser removidos."""
        result = slugify('My Decision (v2)!')
        self.assertEqual(result, 'my-decision-v2')

    def test_slugify_lowercase(self):
        result = slugify('ALL CAPS TITLE')
        self.assertEqual(result, 'all-caps-title')

    def test_slugify_hifens_multiplos(self):
        """Hifens múltiplos consecutivos devem ser colapsados."""
        result = slugify('foo  bar')
        self.assertEqual(result, 'foo-bar')


class TestGenerateAdr(unittest.TestCase):

    def test_generate_adr_cria_arquivo(self):
        """generate_adr deve criar o arquivo com nome baseado em data e frontmatter canônico."""
        with tempfile.TemporaryDirectory() as tmpdir:
            adr_dir = os.path.join(tmpdir, 'docs', 'adr')
            filepath = generate_adr(
                title='Minha Decisão Técnica',
                status='Draft',
                adr_dirs=[adr_dir],
                cwd=tmpdir,
            )

            self.assertTrue(os.path.isfile(filepath))

            # Nome do arquivo: ADR-YYYY-MM-DD-<slug>.md
            basename = os.path.basename(filepath)
            self.assertRegex(basename, r'^ADR-\d{4}-\d{2}-\d{2}-minha-decisao-tecnica\.md$')

            with open(filepath, encoding='utf-8') as f:
                content = f.read()

            today = date.today().isoformat()
            # Frontmatter canônico
            self.assertIn('status: Draft', content)
            self.assertIn(f'date: {today}', content)
            self.assertIn('author: ""', content)
            # Header inline
            self.assertIn(f'> Date: {today} | Status: Draft', content)
            # H1 canônico
            self.assertIn('# ADR: Minha Decisão Técnica', content)
            # Seções canônicas
            self.assertIn('## Context', content)
            self.assertIn('## Decision', content)
            self.assertIn('## Consequences', content)
            self.assertIn('## Alternatives Considered', content)

    def test_generate_adr_nomes_com_data(self):
        """Dois ADRs gerados no mesmo diretório devem ter nomes baseados em data."""
        with tempfile.TemporaryDirectory() as tmpdir:
            adr_dir = os.path.join(tmpdir, 'docs', 'adr')
            today = date.today().isoformat()

            path1 = generate_adr(
                title='Primeira Decisão',
                adr_dirs=[adr_dir],
                cwd=tmpdir,
            )
            path2 = generate_adr(
                title='Segunda Decisão',
                adr_dirs=[adr_dir],
                cwd=tmpdir,
            )

            name1 = os.path.basename(path1)
            name2 = os.path.basename(path2)

            # Ambos devem conter a data atual no nome
            self.assertIn(today, name1)
            self.assertIn(today, name2)
            # Slugs distintos
            self.assertIn('primeira-decisao', name1)
            self.assertIn('segunda-decisao', name2)

    def test_generate_adr_status_padrao_proposed(self):
        """Status padrão deve ser 'Proposed' (canônico Go/Node)."""
        with tempfile.TemporaryDirectory() as tmpdir:
            adr_dir = os.path.join(tmpdir, 'docs', 'adr')
            filepath = generate_adr(
                title='Decisão Sem Status',
                adr_dirs=[adr_dir],
                cwd=tmpdir,
            )
            with open(filepath, encoding='utf-8') as f:
                content = f.read()
            self.assertIn('status: Proposed', content)

    def test_generate_adr_cria_dir_se_inexistente(self):
        """O diretório de ADRs deve ser criado automaticamente."""
        with tempfile.TemporaryDirectory() as tmpdir:
            adr_dir = os.path.join(tmpdir, 'docs', 'adr', 'subdir')
            filepath = generate_adr(
                title='Test Dir Creation',
                adr_dirs=[adr_dir],
                cwd=tmpdir,
            )
            self.assertTrue(os.path.isfile(filepath))


class TestGlobalAdrDir(unittest.TestCase):

    def test_global_adr_dir_junta_home_trackfw_adr(self):
        """global_adr_dir deve retornar <home>/.trackfw/adr, mesmo padrão de
        GlobalADRDir (Go) e do path literal usado em npm/src/commands/adr.js."""
        result = global_adr_dir('/fake/home')
        self.assertEqual(result, os.path.join('/fake/home', '.trackfw', 'adr'))


class TestListAdrs(unittest.TestCase):

    def test_list_adrs_dir_inexistente(self):
        """Diretório ausente deve imprimir 'No ADRs found in <dir>', sem erro."""
        with tempfile.TemporaryDirectory() as tmpdir:
            missing_dir = os.path.join(tmpdir, 'nao-existe')
            buf = io.StringIO()
            with redirect_stdout(buf):
                list_adrs(missing_dir)
            self.assertIn(f'No ADRs found in {missing_dir}', buf.getvalue())

    def test_list_adrs_dir_vazio(self):
        """Diretório existente sem .md deve imprimir a mesma mensagem de 'não encontrado'."""
        with tempfile.TemporaryDirectory() as tmpdir:
            buf = io.StringIO()
            with redirect_stdout(buf):
                list_adrs(tmpdir)
            self.assertIn(f'No ADRs found in {tmpdir}', buf.getvalue())

    def test_list_adrs_formato_filename_status(self):
        """Cada linha deve ser '<filename padded a 60> <status>', em ordem alfabética —
        formato byte-a-byte comparável a ListADRs (Go) / listADRs (Node)."""
        with tempfile.TemporaryDirectory() as tmpdir:
            generate_adr(title='Zeta Decision', status='Accepted', adr_dirs=[tmpdir], cwd=tmpdir)
            generate_adr(title='Alpha Decision', status='Draft', adr_dirs=[tmpdir], cwd=tmpdir)

            buf = io.StringIO()
            with redirect_stdout(buf):
                list_adrs(tmpdir)
            lines = [line for line in buf.getvalue().splitlines() if line]

            self.assertEqual(len(lines), 2)
            # Ordem alfabética de filename (mesma data no nome -> ordena por slug)
            self.assertTrue(lines[0].startswith('ADR-') and 'alpha-decision' in lines[0])
            self.assertTrue(lines[1].startswith('ADR-') and 'zeta-decision' in lines[1])
            self.assertTrue(lines[0].endswith('Draft'))
            self.assertTrue(lines[1].endswith('Accepted'))
            # Filename ocupa exatamente 60 colunas antes do status (padEnd/%-60s)
            filename0 = lines[0].rsplit(' ', 1)[0].rstrip()
            self.assertEqual(lines[0][:60], f'{filename0:<60}')


class TestAdrCommandScope(unittest.TestCase):
    """Testa `trackfw adr new`/`adr list` com --scope, via trackfw.commands.adr diretamente
    (sem subprocess). $HOME é sempre isolado em tmp_dir — nunca o $HOME real da máquina."""

    def setUp(self):
        trackfw_config.reset()
        self._orig_home = os.environ.get('HOME')
        self._orig_cwd = os.getcwd()

    def tearDown(self):
        trackfw_config.reset()
        if self._orig_home is not None:
            os.environ['HOME'] = self._orig_home
        os.chdir(self._orig_cwd)

    def test_scope_global_cria_arquivo_em_home_trackfw_adr(self):
        """--scope global deve criar o ADR em $HOME/.trackfw/adr/, sem exigir trackfw.yaml no cwd."""
        with tempfile.TemporaryDirectory() as fake_home, \
                tempfile.TemporaryDirectory() as no_project_cwd, \
                _chdir(no_project_cwd):  # cwd SEM trackfw.yaml
            os.environ['HOME'] = fake_home

            args = argparse.Namespace(
                title='Decisao Global de Teste',
                status='Proposed',
                dir=None,
                scope='global',
            )
            buf = io.StringIO()
            with redirect_stdout(buf):
                adr_cmd._cmd_new(args)

            expected_dir = os.path.join(fake_home, '.trackfw', 'adr')
            created = [f for f in os.listdir(expected_dir) if f.endswith('.md')]
            self.assertEqual(len(created), 1)
            self.assertIn('decisao-global-de-teste', created[0])
            self.assertIn(os.path.join(expected_dir, created[0]), buf.getvalue())

    def test_scope_project_default_comportamento_atual_preservado(self):
        """--scope project (default) deve continuar idêntico: usa adr_dirs do trackfw.yaml
        (ou docs/adr por padrão), comportamento inalterado por esta feature."""
        with tempfile.TemporaryDirectory() as project_dir, _chdir(project_dir):
            args = argparse.Namespace(
                title='Decisao De Projeto',
                status='Proposed',
                dir=None,
                scope='project',
            )
            adr_cmd._cmd_new(args)

            expected_dir = os.path.join(project_dir, 'docs', 'adr')
            created = [f for f in os.listdir(expected_dir) if f.endswith('.md')]
            self.assertEqual(len(created), 1)
            self.assertIn('decisao-de-projeto', created[0])

    def test_scope_global_com_dir_da_erro_claro(self):
        """--scope global + --dir juntos devem falhar com mensagem clara, não silenciar um dos dois."""
        with tempfile.TemporaryDirectory() as fake_home:
            os.environ['HOME'] = fake_home
            args = argparse.Namespace(
                title='Nao Deve Criar',
                status='Proposed',
                dir='/algum/dir/explicito',
                scope='global',
            )
            with self.assertRaises(SystemExit) as ctx:
                adr_cmd._cmd_new(args)
            self.assertNotEqual(ctx.exception.code, 0)

    def test_adr_list_scope_global(self):
        """`adr list --scope global` lista os ADRs criados em $HOME/.trackfw/adr/."""
        with tempfile.TemporaryDirectory() as fake_home, \
                tempfile.TemporaryDirectory() as no_project_cwd, \
                _chdir(no_project_cwd):
            os.environ['HOME'] = fake_home

            new_args = argparse.Namespace(
                title='ADR Listado Global', status='Proposed', dir=None, scope='global',
            )
            adr_cmd._cmd_new(new_args)

            list_args = argparse.Namespace(scope='global')
            buf = io.StringIO()
            with redirect_stdout(buf):
                adr_cmd._cmd_list(list_args)

            self.assertIn('adr-listado-global', buf.getvalue())
            self.assertIn('Proposed', buf.getvalue())

    def test_adr_list_scope_project(self):
        """`adr list --scope project` (default) lista os ADRs de docs/adr no cwd atual."""
        with tempfile.TemporaryDirectory() as project_dir, _chdir(project_dir):
            new_args = argparse.Namespace(
                title='ADR Listado Projeto', status='Proposed', dir=None, scope='project',
            )
            adr_cmd._cmd_new(new_args)

            list_args = argparse.Namespace(scope='project')
            buf = io.StringIO()
            with redirect_stdout(buf):
                adr_cmd._cmd_list(list_args)

            self.assertIn('adr-listado-projeto', buf.getvalue())
            self.assertIn('Proposed', buf.getvalue())



class TestChdirRestauraAntesDaLimpezaDoTmpdir(unittest.TestCase):
    """ML-4B — falsificacao NAS DUAS DIRECOES da ordem do `with` em `_chdir`.

    O WinError 32 em si so reproduz no Windows (em POSIX um diretorio e removivel
    enquanto e o cwd). O que E falsificavel aqui, em qualquer SO, e a PROPRIEDADE que
    remove a causa: quando a limpeza do tmpdir roda, o cwd do processo ja esta fora dele.
    """

    @contextmanager
    def _tmpdir_que_observa_o_cwd_na_limpeza(self, observed):
        d = tempfile.mkdtemp()
        try:
            yield d
        finally:
            observed.append(os.getcwd())
            shutil.rmtree(d, ignore_errors=True)

    def test_ordem_correta_cwd_ja_esta_fora_quando_a_limpeza_roda(self):
        """`_chdir` por ULTIMO na cadeia: sai primeiro, cwd restaurado antes da limpeza."""
        observed = []
        with self._tmpdir_que_observa_o_cwd_na_limpeza(observed) as d, _chdir(d):
            self.assertEqual(os.path.realpath(os.getcwd()), os.path.realpath(d))
        self.assertEqual(len(observed), 1)
        self.assertNotEqual(
            os.path.realpath(observed[0]), os.path.realpath(d),
            "o cwd ainda estava dentro do tmpdir na limpeza — e o estado que da WinError 32",
        )

    def test_ordem_invertida_deixa_o_cwd_preso_dentro_do_tmpdir(self):
        """Direcao oposta: com a limpeza acontecendo DENTRO do `_chdir` (o que a ordem
        invertida produz), o cwd ainda e o tmpdir — a condicao exata do WinError 32."""
        d = tempfile.mkdtemp()
        observed = []
        try:
            with _chdir(d):
                observed.append(os.getcwd())
            self.assertEqual(os.path.realpath(observed[0]), os.path.realpath(d))
        finally:
            shutil.rmtree(d, ignore_errors=True)


if __name__ == '__main__':
    unittest.main()
