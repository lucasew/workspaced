package modfile

import (
	"fmt"
	"path/filepath"
	"strings"
	"workspaced/pkg/configcue"
)

func ResolveModuleFromConfig(cfg *configcue.Config, moduleName string, modCfg configcue.ModuleEntry, modulesBaseDir string, sumFile *SumFile) (ResolvedModuleSource, error) {
	if from := strings.TrimSpace(modCfg.From); from != "" {
		modFile, err := ModFileFromConfig(cfg)
		if err != nil {
			return ResolvedModuleSource{}, err
		}
		return modFile.ResolveModuleSource(moduleName, from, modulesBaseDir, sumFile)
	}

	inputName := strings.TrimSpace(modCfg.Input)
	if inputName == "" {
		inputName = "self"
	}

	modulePath := strings.Trim(strings.TrimSpace(modCfg.Path), "/")

	// Accept combined specs like "self:modules/base16" in the input field and
	// canonicalize them through the same resolver used everywhere else.
	if strings.Contains(inputName, ":") {
		modFile, err := ModFileFromConfig(cfg)
		if err != nil {
			return ResolvedModuleSource{}, err
		}
		spec := inputName
		if modulePath != "" {
			spec = strings.TrimRight(spec, "/") + ":" + modulePath
		}
		return modFile.ResolveModuleSource(moduleName, spec, modulesBaseDir, sumFile)
	}

	if modulePath == "" {
		modulePath = filepath.ToSlash(filepath.Join("modules", moduleName))
	}

	if inputName == "self" {
		resolved, err := applyVersionLock(moduleName, "self", modulePath, "", sumFile)
		if err != nil {
			return ResolvedModuleSource{}, err
		}
		return resolved, validateNonVersionedProvider(resolved)
	}

	if cfg == nil {
		return ResolvedModuleSource{}, fmt.Errorf("module %q references input %q without config", moduleName, inputName)
	}
	inputs, err := cfg.Inputs()
	if err != nil {
		return ResolvedModuleSource{}, fmt.Errorf("failed to decode inputs: %w", err)
	}
	input, ok := inputs[inputName]
	if !ok {
		return ResolvedModuleSource{}, fmt.Errorf("module %q references unknown input %q", moduleName, inputName)
	}
	spec := strings.TrimSpace(input.From)
	if spec == "" {
		return ResolvedModuleSource{}, fmt.Errorf("input %q is missing from", inputName)
	}
	modFile, err := ModFileFromConfig(cfg)
	if err != nil {
		return ResolvedModuleSource{}, err
	}
	return modFile.ResolveModuleSource(moduleName, spec+":"+modulePath, modulesBaseDir, sumFile)
}
