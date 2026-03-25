package modfile

import (
	"fmt"
	"path/filepath"
	"strings"
	"workspaced/pkg/config"
)

func ResolveModuleFromConfig(cfg *config.GlobalConfig, moduleName string, modCfg map[string]any, modulesBaseDir string, sumFile *SumFile) (ResolvedModuleSource, error) {
	if from, _ := modCfg["from"].(string); strings.TrimSpace(from) != "" {
		modFile, err := ModFileFromConfig(cfg)
		if err != nil {
			return ResolvedModuleSource{}, err
		}
		return modFile.ResolveModuleSource(moduleName, from, modulesBaseDir, sumFile)
	}

	inputName, _ := modCfg["input"].(string)
	inputName = strings.TrimSpace(inputName)
	if inputName == "" {
		inputName = "self"
	}

	modulePath, _ := modCfg["path"].(string)
	modulePath = strings.Trim(strings.TrimSpace(modulePath), "/")
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
	input, ok := cfg.Inputs[inputName]
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
