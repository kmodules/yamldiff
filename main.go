/*
Copyright The Kmodules Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"sort"

	"kmodules.xyz/client-go/tools/parser"

	"github.com/pkg/errors"
	"github.com/spf13/cobra"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"sigs.k8s.io/yaml"
)

func main() {
	rootCmd := &cobra.Command{
		Use:   "yamldiff from.yaml to.yaml",
		Short: "Diff 2 YAML files",
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) != 2 {
				return fmt.Errorf("usage: yamldiff from.json to.json")
			}

			return diff(args[0], args[1])
		},
	}
	rootCmd.Flags().AddGoFlagSet(flag.CommandLine)
	utilruntime.Must(flag.CommandLine.Parse([]string{}))

	utilruntime.Must(rootCmd.Execute())
}

type key struct {
	Group     string
	Version   string
	Kind      string
	Name      string
	Namespace string
}

func diff(fromFile, toFile string) error {
	fromData, err := os.ReadFile(fromFile)
	if err != nil {
		return errors.Errorf("failed to read file %s. reason: %v", fromFile, err)
	}
	fromResources, err := ListResources(fromData)
	if err != nil {
		return err
	}
	fromKeys := map[key]int{}
	for idx, ri := range fromResources {
		fromKeys[key{
			Group:     ri.Object.GetObjectKind().GroupVersionKind().Group,
			Version:   ri.Object.GetObjectKind().GroupVersionKind().Version,
			Kind:      ri.Object.GetObjectKind().GroupVersionKind().Kind,
			Name:      ri.Object.GetName(),
			Namespace: ri.Object.GetNamespace(),
		}] = idx
	}

	toData, err := os.ReadFile(toFile)
	if err != nil {
		return errors.Errorf("failed to read file %s. reason: %v", fromFile, err)
	}
	toResources, err := ListResources(toData)
	if err != nil {
		return err
	}
	sort.SliceStable(toResources, func(i, j int) bool {
		iFrom, iFound := fromKeys[getKey(toResources[i])]
		jFrom, jFound := fromKeys[getKey(toResources[j])]
		if iFound && jFound {
			return iFrom < jFrom
		}
		return i < j
	})

	fromSortedFile, err := writeFile(fromFile, fromResources)
	if err != nil {
		return err
	}
	toSortedFile, err := writeFile(toFile, toResources)
	if err != nil {
		return err
	}
	// defer os.Remove(toSortedFile.Name())

	fmt.Println("diff", fromSortedFile, toSortedFile)
	return nil

	// return sh.Command("diff", fromFile, toSortedFile.Name()).Run()
}

func writeFile(toFile string, resources []parser.ResourceInfo) (string, error) {
	sortedFile, err := os.CreateTemp("/tmp", filepath.Base(toFile))
	if err != nil {
		return "", err
	}
	for idx, ri := range resources {
		if idx > 0 {
			_, err := sortedFile.WriteString("---\n")
			if err != nil {
				return "", err
			}
		}
		data, err := yaml.Marshal(ri.Object)
		if err != nil {
			return "", err
		}
		_, err = sortedFile.Write(data)
		if err != nil {
			return "", err
		}
	}
	err = sortedFile.Close()
	if err != nil {
		return "", err
	}
	return sortedFile.Name(), nil
}

func ListResources(data []byte) ([]parser.ResourceInfo, error) {
	var resources []parser.ResourceInfo
	err := parser.ProcessResources(data, func(ri parser.ResourceInfo) error {
		if ri.Object.GetNamespace() == "" {
			ri.Object.SetNamespace("default")
		}
		resources = append(resources, ri)
		return nil
	})
	if err != nil {
		return nil, err
	}
	return resources, nil
}

func getKey(ri parser.ResourceInfo) key {
	return key{
		Group:     ri.Object.GetObjectKind().GroupVersionKind().Group,
		Version:   ri.Object.GetObjectKind().GroupVersionKind().Version,
		Kind:      ri.Object.GetObjectKind().GroupVersionKind().Kind,
		Name:      ri.Object.GetName(),
		Namespace: ri.Object.GetNamespace(),
	}
}
