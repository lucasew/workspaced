package lsp

import "encoding/json"

// advertisedCapabilities is the full proxy surface ("announce all").
// Methods without a live backend return MethodNotFound / null as appropriate.
func advertisedCapabilities() json.RawMessage {
	// Full textDocumentSync (simplest reliable route for multi-backend replay).
	const caps = `{
		"textDocumentSync": {
			"openClose": true,
			"change": 1,
			"save": { "includeText": true }
		},
		"hoverProvider": true,
		"completionProvider": {
			"triggerCharacters": [".", ":", "/", "\"", "'"],
			"resolveProvider": true
		},
		"signatureHelpProvider": {
			"triggerCharacters": ["(", ","]
		},
		"definitionProvider": true,
		"typeDefinitionProvider": true,
		"implementationProvider": true,
		"referencesProvider": true,
		"documentHighlightProvider": true,
		"documentSymbolProvider": true,
		"workspaceSymbolProvider": true,
		"codeActionProvider": true,
		"codeLensProvider": { "resolveProvider": true },
		"documentFormattingProvider": true,
		"documentRangeFormattingProvider": true,
		"renameProvider": { "prepareProvider": true },
		"documentLinkProvider": { "resolveProvider": true },
		"foldingRangeProvider": true,
		"selectionRangeProvider": true,
		"inlayHintProvider": true,
		"diagnosticProvider": {
			"interFileDependencies": true,
			"workspaceDiagnostics": false
		},
		"semanticTokensProvider": {
			"legend": {
				"tokenTypes": ["namespace","type","class","enum","interface","struct","typeParameter","parameter","variable","property","enumMember","event","function","method","macro","keyword","modifier","comment","string","number","regexp","operator"],
				"tokenModifiers": ["declaration","definition","readonly","static","deprecated","abstract","async","modification","documentation","defaultLibrary"]
			},
			"full": true,
			"range": true
		},
		"workspace": {
			"workspaceFolders": {
				"supported": false
			}
		}
	}`
	return json.RawMessage(caps)
}

// initializeResult builds the initialize response.
func initializeResult(rootURI string) map[string]any {
	return map[string]any{
		"capabilities": advertisedCapabilities(),
		"serverInfo": map[string]any{
			"name":    "workspaced",
			"version": "lsp-router",
		},
	}
}
