package main

import (
	"strings"

	agentcomposev2 "agent-compose/proto/agentcompose/v2"
)

type composePSProjectSelection struct {
	composePath string
	projectName string
	projectRef  *agentcomposev2.ProjectRef
}

// resolveComposePSProject selects an already-applied project by name only when
// no default compose file exists. Explicit files and discovered compose files
// retain the ordinary compose project identity rules.
func resolveComposePSProject(cli cliOptions) (composePSProjectSelection, error) {
	projectName := strings.TrimSpace(cli.ProjectName)
	if strings.TrimSpace(cli.ComposeFile) == "" && projectName != "" {
		composePath, err := resolveComposePath("")
		if err != nil {
			return composePSProjectSelection{}, err
		}
		exists, err := fileExists(composePath)
		if err != nil {
			return composePSProjectSelection{}, err
		}
		if !exists {
			return composePSProjectSelection{
				projectName: projectName,
				projectRef:  &agentcomposev2.ProjectRef{Name: projectName},
			}, nil
		}
	}

	composePath, normalized, projectID, err := resolveComposeProject(cli)
	if err != nil {
		return composePSProjectSelection{}, err
	}
	return composePSProjectSelection{
		composePath: composePath,
		projectName: normalized.Name,
		projectRef:  &agentcomposev2.ProjectRef{ProjectId: projectID},
	}, nil
}
