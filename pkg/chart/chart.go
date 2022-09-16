package chart

import (
	"encoding/json"
	"fmt"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/aiyengar2/hull/pkg/writer"
	"github.com/iancoleman/strcase"
	"github.com/invopop/jsonschema"
	"github.com/rancher/helm-locker/pkg/objectset/parser"
	"github.com/rancher/wrangler/pkg/objectset"
	"github.com/stretchr/testify/assert"
	helmChart "helm.sh/helm/v3/pkg/chart"
	helmLoader "helm.sh/helm/v3/pkg/chart/loader"
	helmChartUtil "helm.sh/helm/v3/pkg/chartutil"
	helmEngine "helm.sh/helm/v3/pkg/engine"
)

type Chart interface {
	GetPath() string
	GetHelmChart() *helmChart.Chart

	RenderTemplate(opts *TemplateOptions) (Template, error)

	MatchesValuesSchema(t *testing.T, schemaStruct interface{})
}

type chart struct {
	*helmChart.Chart

	Path string
}

func NewChart(path string) (Chart, error) {
	absPath, err := filepath.Abs(path)
	if err != nil {
		return nil, err
	}
	c := &chart{
		Path: absPath,
	}
	c.Chart, err = helmLoader.Load(c.Path)
	if err != nil {
		return nil, err
	}

	return c, nil
}

func (c *chart) GetPath() string {
	return c.Path
}

func (c *chart) GetHelmChart() *helmChart.Chart {
	return c.Chart
}

func (c *chart) RenderTemplate(opts *TemplateOptions) (Template, error) {
	opts = opts.setDefaults(c.Metadata.Name)
	values, err := opts.ValuesOptions.MergeValues(nil)
	if err != nil {
		return nil, err
	}
	renderValues, err := helmChartUtil.ToRenderValues(c.Chart, values, opts.Release, opts.Capabilities)
	if err != nil {
		return nil, err
	}
	e := helmEngine.Engine{LintMode: true}
	templateYamls, err := e.Render(c.Chart, renderValues)
	if err != nil {
		return nil, err
	}
	files := make(map[string]string)
	objectsets := map[string]*objectset.ObjectSet{
		"": objectset.NewObjectSet(),
	}
	for source, manifestString := range templateYamls {
		if filepath.Ext(source) != ".yaml" {
			continue
		}
		source := strings.SplitN(source, string(filepath.Separator), 2)[1]
		manifestString := fmt.Sprintf("---\n%s", manifestString)
		manifestOs, err := parser.Parse(manifestString)
		if err != nil {
			return nil, err
		}
		files[source] = manifestString
		objectsets[source] = manifestOs
		objectsets[""] = objectsets[""].Add(manifestOs.All()...)
	}
	t := &template{
		Options:    opts,
		Files:      files,
		ObjectSets: objectsets,
		Values:     values,
	}
	t.Chart = c
	return t, nil
}

func (c *chart) MatchesValuesSchema(t *testing.T, schemaStruct interface{}) {
	if c.Chart.Schema == nil {
		t.Errorf("chart does not have schema")
		return
	}

	r := &jsonschema.Reflector{
		DoNotReference: true,
		Namer: func(t reflect.Type) string {
			return strcase.ToLowerCamel(t.Name())
		},
		KeyNamer: strcase.ToLowerCamel,
	}
	expectedSchema := r.Reflect(schemaStruct)
	expectedSchemaBytes, err := json.MarshalIndent(expectedSchema, "", "  ")
	if err != nil {
		t.Error(err)
		return
	}
	expectedSchemaBytes = append(expectedSchemaBytes, '\n')

	assert.Equal(t, string(expectedSchemaBytes), string(c.Chart.Schema))
	if !t.Failed() {
		return
	}

	// Write to output file
	w := writer.NewChartPathWriter(
		t,
		c.Chart.Metadata.Name,
		c.Chart.Metadata.Version,
		"values.schema.json",
		fmt.Sprintf("jsonschema.Reflect(%T)", schemaStruct),
		string(c.Chart.Schema),
	)
	if _, err := w.Write(expectedSchemaBytes); err != nil {
		t.Error(err)
		return
	}
}
