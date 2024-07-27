package main

import (
	"fmt"
	"log"
	"os"
	"os/exec"

	jsonpatch "github.com/evanphx/json-patch"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/strategicpatch"
	"k8s.io/apimachinery/pkg/util/yaml"
	"k8s.io/kubectl/pkg/scheme"
)

func main() {
	log.SetFlags(0)

	var args []string
	args = append(args, "get", "-o", "yaml")
	args = append(args, os.Args[1:]...)
	b, err := exec.Command("kubectl", args...).CombinedOutput()
	if err != nil {
		log.Fatalf("%s", b)
	}
	t, err := os.CreateTemp("", "*.yaml")
	if err != nil {
		log.Fatalf("Failed to create tempfile: %+v", err.Error())
	}
	if err := os.WriteFile(t.Name(), b, 0644); err != nil {
		log.Fatalf("Failed to write to tempfile: %+v", err)
	}
	var editor string
	editor, ok := os.LookupEnv("EDITOR")
	if !ok {
		editor = "vim"
	}
	cmd := exec.Command(editor, t.Name())
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		log.Fatalf("Failed to edit tempfile: %+v", err)
	}
	oldjson, err := yaml.ToJSON(b)
	if err != nil {
		log.Fatalf("Failed to marshal json: %+v", err)
	}

	tb, err := os.ReadFile(t.Name())
	if err != nil {
		log.Fatalf("Failed to read tempfile: %+v", err)
	}
	newjson, err := yaml.ToJSON(tb)
	if err != nil {
		log.Fatalf("Failed to marshal json: %+v", err)
	}

	unstructuredobject, err := runtime.Decode(unstructured.UnstructuredJSONScheme, oldjson)
	if err != nil {
		log.Fatalf("Failed to decode kubernetes object: %+v", err)
	}
	var patch []byte
	object, err := scheme.Scheme.New(unstructuredobject.GetObjectKind().GroupVersionKind())
	if err == nil {
		patch, err = strategicpatch.CreateTwoWayMergePatch(oldjson, newjson, object)
		if err != nil {
			log.Fatalf("Failed to create jsonpatch: %+v", err)
		}
	} else {
		patch, err = jsonpatch.CreateMergePatch(oldjson, newjson)
		if err != nil {
			log.Fatalf("Failed to create jsonpatch: %+v", err)
		}
	}
	fmt.Println(string(patch))
}
