package integrations

// legacyHashes contains only byte-identical artifacts produced by released
// installers whose historical destination collides with a canonical one.
// Historical artifacts written to different paths are deliberately untouched.
var legacyHashes = map[string][]string{
	"claude\x00cli\x00global\x00agents\x00architect":    {"d28ae507d2ce9fd3fcd7cb1a0c1ffaaebc994dc9c45b219e5361b909dc6132ba"},
	"claude\x00cli\x00global\x00agents\x00backend":      {"587bf790907bc7451976c026b9c7dc5419541a5fdfb064586744198dcf8c0439"},
	"claude\x00cli\x00global\x00agents\x00code-quality": {"8917909b55ed54cfa8bb66015a301428f4d924bb3658a9c4c5d443e14e60d399"},
	"claude\x00cli\x00global\x00agents\x00data":         {"3a402a85610cf3d463200d0f81fda61b5c6c02a0198d302a5d6da49e8e5ff688"},
	"claude\x00cli\x00global\x00agents\x00dba":          {"07aa92c09833a7c72c3d9b3e39ec1272a91e435deda2da5ddef321abed3702fa"},
	"claude\x00cli\x00global\x00agents\x00frontend":     {"7ae3940a0827b0047363cd984be5e96b66902098c5ea669d0975a234027ca39c"},
	"claude\x00cli\x00global\x00agents\x00infra":        {"9f5534c83c3d3b0a9e8c550bd7a3cbb861eb7c4d7671fe8783cd68ff5d5c1605"},
	"claude\x00cli\x00global\x00agents\x00qa":           {"384283eb46d5e3c5ee978fc87e1b5cd44009e3c62d57c8dcd5cf92189b79e291"},
	"claude\x00cli\x00global\x00agents\x00security":     {"10e02aa03fe502174174e376d9ec845c549f828af885059214af23854efb4c3f"},
	"claude\x00cli\x00global\x00agents\x00ux":           {"49b5bbcb8063075bda0b254d1040ae7d2b8d8c315f538cf05e39774ab8a2907b"},

	// Codex reviewer has no canonical item and is intentionally not adopted.
	// Each Codex entry is the union of Go, npm and PyPI historical bytes, so
	// changing the package used to run trackfw does not break safe adoption.
	"codex\x00cli\x00project\x00agents\x00architect":  {"c7e4b34984a8ba54753ecb9c0b2a1ad10b0d2083a15e03bcb19016012efa39e1", "38199d6edfc0fd7c5d6663996541a25a17203d87713709d0a0daad2ecc5d6be6", "94512bd3db605d5170841018adc2246ce314a57b3be07938fc61d0c41ef6126c"},
	"codex\x00cli\x00project\x00agents\x00backend":    {"0e2327ace2c719fcb0abe2f17dfdbad4a0d987824f888e6802a9f4a132163f64", "348c5ab0a57597f332c2a5b293f045a0d3980875a9c9083a06cad15f909e23a7", "43759e9a912bb236a997d58ad184dfcda2b45f6e00ed5a5edad93d83dd9c906d"},
	"codex\x00cli\x00project\x00agents\x00frontend":   {"1a377e6c9a51f1df549e475558897dac428f94ae50e6955a1ca99cb0b04b0648", "608c10cc0f910725ad813736a2548e1c66479abce28d32d4b8969707be1054f1", "5ebf3a404d300e17338538eeb8f50d63bda8fde5f5f0bf0b73767434233ae9c4"},
	"codex\x00cli\x00project\x00agents\x00qa":         {"aad7e86d59e511e6b1519e2e26bb4de376eb124725a44d82f704edf396989ac0", "3e8b3a5d04aa44c62ce6af801be9d907aa6458931c19676ca999776ac961254d", "33663bb0ee4bee4cd971a691283649b308ad093a01000762a70ec79481d10e52"},
	"codex\x00cli\x00project\x00agents\x00security":   {"b7743d40163066794941f37fab259f965bb78c54c9a8fe3cc7a0146d2552f016", "cdd197001ebabf8cf63b8c1562d0eb89b226df8a2e5dcb1ce84e9a1ea6de782d", "1e3987835233bbdb0751e98a50ec7512dfc7855e293aa40d1ad77c80dde1945f"},
	"codex\x00cli\x00project\x00skills\x00governance": {"54edf417d53e2a91f52651de5a31a9c1b80be8d0f7f1252c5b9f3b55630a9b96", "c30786495105715ae707d374b414d21344b20a0cd354713f8173c5109d690fd9", "be044d1a1f2af4835d476a3f4798b35e2067eeebdff8f30872353eee6b212736"},
	"codex\x00cli\x00project\x00skills\x00plan":       {"dc038a9f18bd0fee7e03d8c7816f058e9c95a4e729dddfb086d642a61bc676ba", "2ef36f3ac8c77eb46d13ba4cbe420e1c872f71edace9b7851d63cb50bc5f3708", "b2cd10338db33369e78656a1e3877427a454f9eab6fa1ec948fd17730c66b46a"},
	"codex\x00cli\x00project\x00skills\x00implement":  {"bb9bbe197244cd4537610579b324757e8c7f97755a78fdb01daab4638485bfc2", "898682d21e6ba938a2ef2fe4908f4047358e3f27ca291068a5585a47aca1426e", "c5a53c5e9444810d3beeebe563798de792615c7703a0d99e954d578d068af4d4"},
	"codex\x00cli\x00project\x00skills\x00review":     {"bea0debcb3b8f5014a7c7ae0e93b9b8c4b318e14c8fc8ff6510594410a123783", "b17fbc6d59bdfad64d2c1993cc588c0e6e9ae8e4b4a16669abf254d41cc40330", "8b3119641a0d06b99b451cf6437c15a2f7f58069fb0ebf855ab757f66b919b53"},
	"codex\x00cli\x00project\x00skills\x00release":    {"2db9c460046da9cb918acb7f716727a07fdb5b361f2a81a67c35b8fe09135fcd", "2450fd34b81a14d28c794abc0e9ac0213ba929a950d15b3226443c92419d42c9"},
}

func LegacyHashes(claim Claim) []string {
	key := claim.Target + "\x00" + claim.Surface + "\x00" + claim.Scope + "\x00" + string(claim.Kind) + "\x00" + claim.Item
	return append([]string(nil), legacyHashes[key]...)
}
