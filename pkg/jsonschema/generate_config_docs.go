package jsonschema

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/iancoleman/orderedmap"
	"github.com/jesseduffield/lazycore/pkg/utils"

	"gopkg.in/yaml.v3"
)

type Node struct {
	Name        string
	Description string
	Default     any
	Children    []*Node
}

const IndentLevel = 2

func (n *Node) MarshalYAML() (interface{}, error) {
	node := yaml.Node{
		Kind: yaml.MappingNode,
	}

	keyNode := yaml.Node{
		Kind:  yaml.ScalarNode,
		Value: n.Name,
	}
	if n.Description != "" {
		keyNode.HeadComment = n.Description
	}

	if n.Default != nil {
		valueNode := yaml.Node{
			Kind: yaml.ScalarNode,
		}
		err := valueNode.Encode(n.Default)
		if err != nil {
			return nil, err
		}
		node.Content = append(node.Content, &keyNode, &valueNode)
	} else if len(n.Children) > 0 {
		childrenNode := yaml.Node{
			Kind: yaml.MappingNode,
		}
		for _, child := range n.Children {
			childYaml, err := child.MarshalYAML()
			if err != nil {
				return nil, err
			}

			childKey := yaml.Node{
				Kind:  yaml.ScalarNode,
				Value: child.Name,
			}
			if child.Description != "" {
				childKey.HeadComment = child.Description
			}
			childYaml = childYaml.(*yaml.Node)
			childrenNode.Content = append(childrenNode.Content, childYaml.(*yaml.Node).Content...)
		}
		node.Content = append(node.Content, &keyNode, &childrenNode)
	}

	return &node, nil
}

func getDescription(v *orderedmap.OrderedMap) string {
	description, ok := v.Get("description")
	if !ok {
		description = ""
	}
	return description.(string)
}

func getDefault(v *orderedmap.OrderedMap) (error, any) {
	defaultValue, ok := v.Get("default")
	if ok {
		return nil, defaultValue
	}

	dataType, ok := v.Get("type")
	if ok {
		dataTypeString := dataType.(string)
		if dataTypeString == "string" {
			return nil, ""
		}
	}

	return fmt.Errorf("Failed to get default value"), nil
}

func parseNode(parent *Node, name string, value *orderedmap.OrderedMap) {
	description := getDescription(value)
	err, defaultValue := getDefault(value)
	if err == nil {
		leaf := &Node{Name: name, Description: description, Default: defaultValue}
		parent.Children = append(parent.Children, leaf)
	}

	properties, ok := value.Get("properties")
	if !ok {
		return
	}

	orderedProperties := properties.(orderedmap.OrderedMap)

	node := &Node{Name: name, Description: description}
	parent.Children = append(parent.Children, node)

	keys := orderedProperties.Keys()
	for _, name := range keys {
		value, _ := orderedProperties.Get(name)
		typedValue := value.(orderedmap.OrderedMap)
		parseNode(node, name, &typedValue)
	}
}

func writeToConfigDocs(buffer *bytes.Buffer) {
	// Remove all `---` lines
	strData := buffer.String()
	lines := strings.Split(strData, "\n")

	var newBuffer bytes.Buffer

	for _, line := range lines {
		if strings.TrimSpace(line) != "---" {
			newBuffer.WriteString(line + "\n")
		}
	}

	config := newBuffer.Bytes()
	config = config[:len(config)-1]

	configPath := utils.GetLazyRootDirectory() + "/docs/Config.md"
	markdown, err := os.ReadFile(configPath)
	if err != nil {
		fmt.Println("Error reading Config.md file")
		return
	}

	defaultSectionIndex := bytes.Index(markdown, []byte("## Default"))
	if defaultSectionIndex == -1 {
		fmt.Println("## Default section not found")
		return
	}

	startCodeBlockIndex := bytes.Index(markdown[defaultSectionIndex:], []byte("```yaml"))
	if startCodeBlockIndex == -1 {
		fmt.Println("```yaml code block not found after ## Default")
		return
	}
	startCodeBlockIndex += defaultSectionIndex

	endCodeBlockIndex := bytes.Index(markdown[startCodeBlockIndex+len("```yaml"):], []byte("```"))
	if endCodeBlockIndex == -1 {
		fmt.Println("Closing ``` not found for code block")
		return
	}
	endCodeBlockIndex += startCodeBlockIndex + len("```yaml")

	newMarkdown := make([]byte, 0, len(markdown)-endCodeBlockIndex+startCodeBlockIndex+len(config))
	newMarkdown = append(newMarkdown, markdown[:startCodeBlockIndex+len("```yaml\n")]...)
	newMarkdown = append(newMarkdown, config...)
	newMarkdown = append(newMarkdown, markdown[endCodeBlockIndex:]...)

	if err := os.WriteFile(configPath, newMarkdown, 0o644); err != nil {
		fmt.Println("Error writing to file:", err)
		return
	}
}

func GenerateConfigDocs() {
	content, err := os.ReadFile(GetSchemaDir() + "/config.json")
	if err != nil {
		panic("Error reading config.json")
	}

	schema := orderedmap.New()

	err = json.Unmarshal(content, &schema)
	if err != nil {
		panic("Failed to unmarshal config.json")
	}

	root, ok := schema.Get("properties")
	if !ok {
		panic("properties key not found in schema")
	}
	orderedRoot := root.(orderedmap.OrderedMap)

	rootNode := Node{}
	for _, name := range orderedRoot.Keys() {
		value, _ := orderedRoot.Get(name)
		typedValue := value.(orderedmap.OrderedMap)
		parseNode(&rootNode, name, &typedValue)
	}

	var buffer bytes.Buffer
	encoder := yaml.NewEncoder(&buffer)
	encoder.SetIndent(IndentLevel)

	for _, child := range rootNode.Children {
		err := encoder.Encode(child)
		if err != nil {
			panic("Failed to Marshal document")
		}
	}

	writeToConfigDocs(&buffer)
}
