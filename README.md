# WorkspaceD

- A modular template system for dotfiles, and a modular driver system to abstract away common system tools
- Gives most of the benefits that Nix/NixOS has, but way faster

## Terminology
- Driver: abstract interface to consume services of the operating system such as audio control, workspace management and cameras. Implementations are added through the registry pattern using the DriverProvider
- Linter: something that points issues in a codebase or a system
- Formatter: something that restructures code to be in a specific structure without altering functionality
- Tool: something that lists available versions and install a scoped program from a version. One tool version normally has assets and at least one executable binary
- Tool backend: something that takes a ref and gives a Tool. Ex: the backend of GitHub takes owner/repo as ref
- Module: a part of the cue configuration that is defined somewhere else, along with the templates of files

# TODO
- [ ] Rename DriverProvider to something less provider
- [ ] Dismember check into linter and formatter?
