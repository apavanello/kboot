package ui

import (
	"fmt"
	"kboot/internal/config"

	"github.com/charmbracelet/huh"
)

// PromptOptionalClusters shows a multi-select for optional clusters
// Returns the list of clusters that should be booted (mandatory + selected optional)
func PromptOptionalClusters(allClusters []config.Cluster) ([]config.Cluster, error) {
	var mandatory []config.Cluster
	var optional []config.Cluster
	var options []huh.Option[string]

	for _, c := range allClusters {
		if c.Optional {
			optional = append(optional, c)
			// Label: "Alias (Profile)"
			label := fmt.Sprintf("%s (%s)", c.Alias, c.Profile)
			options = append(options, huh.NewOption(label, c.Alias))
		} else {
			mandatory = append(mandatory, c)
		}
	}

	if len(optional) == 0 {
		return mandatory, nil
	}

	var selectedAliases []string

	form := huh.NewForm(
		huh.NewGroup(
			huh.NewMultiSelect[string]().
				Title("Clusters Opcionais detectados").
				Description("Selecione quais deseja conectar nesta sessão (Espaço para marcar)").
				Options(options...).
				Value(&selectedAliases),
		),
	)

	err := form.Run()
	if err != nil {
		return nil, err
	}

	// Rebuild final list
	finalList := append([]config.Cluster{}, mandatory...)

	// Map check for speed
	selMap := make(map[string]bool)
	for _, a := range selectedAliases {
		selMap[a] = true
	}

	for _, c := range optional {
		if selMap[c.Alias] {
			finalList = append(finalList, c)
		}
	}

	return finalList, nil
}
