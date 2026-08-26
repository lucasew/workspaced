// Dest file decls. Hosts mount #Tree at a config path via Mount.

#SlotText: close({
	kind: "text"
	text: string
})
#SlotRef: close({
	kind: "ref"
	ref:  string
})
#Slot: string | #SlotText | #SlotRef

#FileLines: close({
	type: "lines"
	values: [string]: #Slot
})
#FileText: close({
	type: "text"
	values: [string]: #Slot
})
#FileRef: close({
	type: "ref"
	values: [string]: #Slot
})
#FileStructured: close({
	type: "json" | "toml" | "yaml" | "ini" | "xml"
	values: [string]: _
})
#File: #FileLines | #FileText | #FileRef | #FileStructured

// Tree is a dest map. Keys are fs.FS names (no leading /, no ~, no ..).
#Tree: [string]: #File
