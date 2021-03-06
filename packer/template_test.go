package packer

import (
	"cgl.tideland.biz/asserts"
	"io/ioutil"
	"os"
	"reflect"
	"sort"
	"testing"
)

func testTemplateComponentFinder() *ComponentFinder {
	builder := testBuilder()
	pp := new(TestPostProcessor)
	provisioner := &MockProvisioner{}

	builderMap := map[string]Builder{
		"test-builder": builder,
	}

	ppMap := map[string]PostProcessor{
		"test-pp": pp,
	}

	provisionerMap := map[string]Provisioner{
		"test-prov": provisioner,
	}

	builderFactory := func(n string) (Builder, error) { return builderMap[n], nil }
	ppFactory := func(n string) (PostProcessor, error) { return ppMap[n], nil }
	provFactory := func(n string) (Provisioner, error) { return provisionerMap[n], nil }
	return &ComponentFinder{
		Builder:       builderFactory,
		PostProcessor: ppFactory,
		Provisioner:   provFactory,
	}
}

func TestParseTemplateFile_basic(t *testing.T) {
	data := `
	{
		"builders": [{"type": "something"}]
	}
	`

	tf, err := ioutil.TempFile("", "packer")
	if err != nil {
		t.Fatalf("err: %s", err)
	}
	tf.Write([]byte(data))
	tf.Close()

	result, err := ParseTemplateFile(tf.Name())
	if err != nil {
		t.Fatalf("err: %s", err)
	}

	if len(result.Builders) != 1 {
		t.Fatalf("bad: %#v", result.Builders)
	}
}

func TestParseTemplateFile_stdin(t *testing.T) {
	data := `
	{
		"builders": [{"type": "something"}]
	}
	`

	tf, err := ioutil.TempFile("", "packer")
	if err != nil {
		t.Fatalf("err: %s", err)
	}
	defer tf.Close()
	tf.Write([]byte(data))

	// Sync and seek to the beginning so that we can re-read the contents
	tf.Sync()
	tf.Seek(0, 0)

	// Set stdin to something we control
	oldStdin := os.Stdin
	defer func() { os.Stdin = oldStdin }()
	os.Stdin = tf

	result, err := ParseTemplateFile("-")
	if err != nil {
		t.Fatalf("err: %s", err)
	}

	if len(result.Builders) != 1 {
		t.Fatalf("bad: %#v", result.Builders)
	}
}

func TestParseTemplate_Basic(t *testing.T) {
	assert := asserts.NewTestingAsserts(t, true)

	data := `
	{
		"builders": [{"type": "something"}]
	}
	`

	result, err := ParseTemplate([]byte(data))
	assert.Nil(err, "should not error")
	assert.NotNil(result, "template should not be nil")
	assert.Length(result.Builders, 1, "one builder")
}

func TestParseTemplate_Invalid(t *testing.T) {
	assert := asserts.NewTestingAsserts(t, true)

	// Note there is an extra comma below for a purposeful
	// syntax error in the JSON.
	data := `
	{
		"builders": [],
	}
	`

	result, err := ParseTemplate([]byte(data))
	assert.NotNil(err, "should have an error")
	assert.Nil(result, "should have no result")
}

func TestParseTemplate_InvalidKeys(t *testing.T) {
	assert := asserts.NewTestingAsserts(t, true)

	// Note there is an extra comma below for a purposeful
	// syntax error in the JSON.
	data := `
	{
		"builders": [{"type": "foo"}],
		"what is this": ""
	}
	`

	result, err := ParseTemplate([]byte(data))
	assert.NotNil(err, "should have an error")
	assert.Nil(result, "should have no result")
}

func TestParseTemplate_BuilderWithoutType(t *testing.T) {
	assert := asserts.NewTestingAsserts(t, true)

	data := `
	{
		"builders": [{}]
	}
	`

	_, err := ParseTemplate([]byte(data))
	assert.NotNil(err, "should have error")
}

func TestParseTemplate_BuilderWithNonStringType(t *testing.T) {
	assert := asserts.NewTestingAsserts(t, true)

	data := `
	{
		"builders": [{
			"type": 42
		}]
	}
	`

	_, err := ParseTemplate([]byte(data))
	assert.NotNil(err, "should have error")
}

func TestParseTemplate_BuilderWithoutName(t *testing.T) {
	assert := asserts.NewTestingAsserts(t, true)

	data := `
	{
		"builders": [
			{
				"type": "amazon-ebs"
			}
		]
	}
	`

	result, err := ParseTemplate([]byte(data))
	assert.Nil(err, "should not error")
	assert.NotNil(result, "template should not be nil")
	assert.Length(result.Builders, 1, "should have one builder")

	builder, ok := result.Builders["amazon-ebs"]
	assert.True(ok, "should have amazon-ebs builder")
	assert.Equal(builder.Type, "amazon-ebs", "builder should be amazon-ebs")
}

func TestParseTemplate_BuilderWithName(t *testing.T) {
	assert := asserts.NewTestingAsserts(t, true)

	data := `
	{
		"builders": [
			{
				"name": "bob",
				"type": "amazon-ebs"
			}
		]
	}
	`

	result, err := ParseTemplate([]byte(data))
	assert.Nil(err, "should not error")
	assert.NotNil(result, "template should not be nil")
	assert.Length(result.Builders, 1, "should have one builder")

	builder, ok := result.Builders["bob"]
	assert.True(ok, "should have bob builder")
	assert.Equal(builder.Type, "amazon-ebs", "builder should be amazon-ebs")

	RawConfig := builder.RawConfig
	if RawConfig == nil {
		t.Fatal("missing builder raw config")
	}

	expected := map[string]interface{}{
		"type": "amazon-ebs",
	}

	if !reflect.DeepEqual(RawConfig, expected) {
		t.Fatalf("bad raw: %#v", RawConfig)
	}
}

func TestParseTemplate_BuilderWithConflictingName(t *testing.T) {
	assert := asserts.NewTestingAsserts(t, true)

	data := `
	{
		"builders": [
			{
				"name": "bob",
				"type": "amazon-ebs"
			},
			{
				"name": "bob",
				"type": "foo",
			}
		]
	}
	`

	_, err := ParseTemplate([]byte(data))
	assert.NotNil(err, "should have error")
}

func TestParseTemplate_Hooks(t *testing.T) {
	assert := asserts.NewTestingAsserts(t, true)

	data := `
	{

		"builders": [{"type": "foo"}],

		"hooks": {
			"event": ["foo", "bar"]
		}
	}
	`

	result, err := ParseTemplate([]byte(data))
	assert.Nil(err, "should not error")
	assert.NotNil(result, "template should not be nil")
	assert.Length(result.Hooks, 1, "should have one hook")

	hooks, ok := result.Hooks["event"]
	assert.True(ok, "should have hook")
	assert.Equal(hooks, []string{"foo", "bar"}, "hooks should be correct")
}

func TestParseTemplate_PostProcessors(t *testing.T) {
	data := `
	{
		"builders": [{"type": "foo"}],

		"post-processors": [
			"simple",

			{ "type": "detailed" },

			[ "foo", { "type": "bar" } ]
		]
	}
	`

	tpl, err := ParseTemplate([]byte(data))
	if err != nil {
		t.Fatalf("error parsing: %s", err)
	}

	if len(tpl.PostProcessors) != 3 {
		t.Fatalf("bad number of post-processors: %d", len(tpl.PostProcessors))
	}

	pp := tpl.PostProcessors[0]
	if len(pp) != 1 {
		t.Fatalf("wrong number of configs in simple: %d", len(pp))
	}

	if pp[0].Type != "simple" {
		t.Fatalf("wrong type for simple: %s", pp[0].Type)
	}

	pp = tpl.PostProcessors[1]
	if len(pp) != 1 {
		t.Fatalf("wrong number of configs in detailed: %d", len(pp))
	}

	if pp[0].Type != "detailed" {
		t.Fatalf("wrong type for detailed: %s", pp[0].Type)
	}

	pp = tpl.PostProcessors[2]
	if len(pp) != 2 {
		t.Fatalf("wrong number of configs for sequence: %d", len(pp))
	}

	if pp[0].Type != "foo" {
		t.Fatalf("wrong type for sequence 0: %s", pp[0].Type)
	}

	if pp[1].Type != "bar" {
		t.Fatalf("wrong type for sequence 1: %s", pp[1].Type)
	}
}

func TestParseTemplate_ProvisionerWithoutType(t *testing.T) {
	assert := asserts.NewTestingAsserts(t, true)

	data := `
	{
		"builders": [{"type": "foo"}],

		"provisioners": [{}]
	}
	`

	_, err := ParseTemplate([]byte(data))
	assert.NotNil(err, "should have error")
}

func TestParseTemplate_ProvisionerWithNonStringType(t *testing.T) {
	assert := asserts.NewTestingAsserts(t, true)

	data := `
	{
		"builders": [{"type": "foo"}],

		"provisioners": [{
			"type": 42
		}]
	}
	`

	_, err := ParseTemplate([]byte(data))
	assert.NotNil(err, "should have error")
}

func TestParseTemplate_Provisioners(t *testing.T) {
	assert := asserts.NewTestingAsserts(t, true)

	data := `
	{
		"builders": [{"type": "foo"}],

		"provisioners": [
			{
				"type": "shell"
			}
		]
	}
	`

	result, err := ParseTemplate([]byte(data))
	assert.Nil(err, "should not error")
	assert.NotNil(result, "template should not be nil")
	assert.Length(result.Provisioners, 1, "should have one provisioner")
	assert.Equal(result.Provisioners[0].Type, "shell", "provisioner should be shell")
	assert.NotNil(result.Provisioners[0].RawConfig, "should have raw config")
}

func TestParseTemplate_Variables(t *testing.T) {
	data := `
	{
		"variables": {
			"foo": "bar",
			"bar": null,
			"baz": 27
		},

		"builders": [{"type": "something"}]
	}
	`

	result, err := ParseTemplate([]byte(data))
	if err != nil {
		t.Fatalf("err: %s", err)
	}

	if result.Variables == nil || len(result.Variables) != 3 {
		t.Fatalf("bad vars: %#v", result.Variables)
	}

	if result.Variables["foo"].Default != "bar" {
		t.Fatal("foo default is not right")
	}

	if result.Variables["foo"].Required {
		t.Fatal("foo should not be required")
	}

	if result.Variables["bar"].Default != "" {
		t.Fatal("default should be empty")
	}

	if !result.Variables["bar"].Required {
		t.Fatal("bar should be required")
	}

	if result.Variables["baz"].Default != "27" {
		t.Fatal("default should be empty")
	}

	if result.Variables["baz"].Required {
		t.Fatal("baz should not be required")
	}
}

func TestParseTemplate_variablesBadDefault(t *testing.T) {
	data := `
	{
		"variables": {
			"foo": 7,
		},

		"builders": [{"type": "something"}]
	}
	`

	_, err := ParseTemplate([]byte(data))
	if err == nil {
		t.Fatal("should have error")
	}
}

func TestTemplate_BuildNames(t *testing.T) {
	assert := asserts.NewTestingAsserts(t, true)

	data := `
	{
		"builders": [
			{
				"name": "bob",
				"type": "amazon-ebs"
			},
			{
				"name": "chris",
				"type": "another"
			}
		]
	}
	`

	result, err := ParseTemplate([]byte(data))
	assert.Nil(err, "should not error")

	buildNames := result.BuildNames()
	sort.Strings(buildNames)
	assert.Equal(buildNames, []string{"bob", "chris"}, "should have proper builds")
}

func TestTemplate_BuildUnknown(t *testing.T) {
	assert := asserts.NewTestingAsserts(t, true)

	data := `
	{
		"builders": [
			{
				"name": "test1",
				"type": "test-builder"
			}
		]
	}
	`

	template, err := ParseTemplate([]byte(data))
	assert.Nil(err, "should not error")

	build, err := template.Build("nope", nil)
	assert.Nil(build, "build should be nil")
	assert.NotNil(err, "should have error")
}

func TestTemplate_BuildUnknownBuilder(t *testing.T) {
	assert := asserts.NewTestingAsserts(t, true)

	data := `
	{
		"builders": [
			{
				"name": "test1",
				"type": "test-builder"
			}
		]
	}
	`

	template, err := ParseTemplate([]byte(data))
	assert.Nil(err, "should not error")

	builderFactory := func(string) (Builder, error) { return nil, nil }
	components := &ComponentFinder{Builder: builderFactory}
	build, err := template.Build("test1", components)
	assert.Nil(build, "build should be nil")
	assert.NotNil(err, "should have error")
}

func TestTemplate_Build_NilBuilderFunc(t *testing.T) {
	assert := asserts.NewTestingAsserts(t, true)

	data := `
	{
		"builders": [
			{
				"name": "test1",
				"type": "test-builder"
			}
		],

		"provisioners": [
			{
				"type": "test-prov"
			}
		]
	}
	`

	template, err := ParseTemplate([]byte(data))
	assert.Nil(err, "should not error")

	defer func() {
		p := recover()
		assert.NotNil(p, "should panic")

		if p != nil {
			assert.Equal(p.(string), "no builder function", "right panic")
		}
	}()

	template.Build("test1", &ComponentFinder{})
}

func TestTemplate_Build_NilProvisionerFunc(t *testing.T) {
	assert := asserts.NewTestingAsserts(t, true)

	data := `
	{
		"builders": [
			{
				"name": "test1",
				"type": "test-builder"
			}
		],

		"provisioners": [
			{
				"type": "test-prov"
			}
		]
	}
	`

	template, err := ParseTemplate([]byte(data))
	assert.Nil(err, "should not error")

	defer func() {
		p := recover()
		assert.NotNil(p, "should panic")

		if p != nil {
			assert.Equal(p.(string), "no provisioner function", "right panic")
		}
	}()

	template.Build("test1", &ComponentFinder{
		Builder: func(string) (Builder, error) { return nil, nil },
	})
}

func TestTemplate_Build_NilProvisionerFunc_WithNoProvisioners(t *testing.T) {
	assert := asserts.NewTestingAsserts(t, true)

	data := `
	{
		"builders": [
			{
				"name": "test1",
				"type": "test-builder"
			}
		],

		"provisioners": []
	}
	`

	template, err := ParseTemplate([]byte(data))
	assert.Nil(err, "should not error")

	template.Build("test1", &ComponentFinder{
		Builder: func(string) (Builder, error) { return nil, nil },
	})
}

func TestTemplate_Build(t *testing.T) {
	assert := asserts.NewTestingAsserts(t, true)

	data := `
	{
		"builders": [
			{
				"name": "test1",
				"type": "test-builder"
			}
		],

		"provisioners": [
			{
				"type": "test-prov"
			}
		],

		"post-processors": [
			"simple",
			[
				"simple",
				{ "type": "simple", "keep_input_artifact": true }
			]
		]
	}
	`

	expectedConfig := map[string]interface{}{
		"type": "test-builder",
	}

	template, err := ParseTemplate([]byte(data))
	assert.Nil(err, "should not error")

	builder := testBuilder()
	builderMap := map[string]Builder{
		"test-builder": builder,
	}

	provisioner := &MockProvisioner{}
	provisionerMap := map[string]Provisioner{
		"test-prov": provisioner,
	}

	pp := new(TestPostProcessor)
	ppMap := map[string]PostProcessor{
		"simple": pp,
	}

	builderFactory := func(n string) (Builder, error) { return builderMap[n], nil }
	ppFactory := func(n string) (PostProcessor, error) { return ppMap[n], nil }
	provFactory := func(n string) (Provisioner, error) { return provisionerMap[n], nil }
	components := &ComponentFinder{
		Builder:       builderFactory,
		PostProcessor: ppFactory,
		Provisioner:   provFactory,
	}

	// Get the build, verifying we can get it without issue, but also
	// that the proper builder was looked up and used for the build.
	build, err := template.Build("test1", components)
	assert.Nil(err, "should not error")

	coreBuild, ok := build.(*coreBuild)
	assert.True(ok, "should be a core build")
	assert.Equal(coreBuild.builder, builder, "should have the same builder")
	assert.Equal(coreBuild.builderConfig, expectedConfig, "should have proper config")
	assert.Equal(len(coreBuild.provisioners), 1, "should have one provisioner")
	assert.Equal(len(coreBuild.postProcessors), 2, "should have pps")
	assert.Equal(len(coreBuild.postProcessors[0]), 1, "should have correct number")
	assert.Equal(len(coreBuild.postProcessors[1]), 2, "should have correct number")
	assert.False(coreBuild.postProcessors[1][0].keepInputArtifact, "shoule be correct")
	assert.True(coreBuild.postProcessors[1][1].keepInputArtifact, "shoule be correct")

	config := coreBuild.postProcessors[1][1].config
	if _, ok := config["keep_input_artifact"]; ok {
		t.Fatal("should not have keep_input_artifact")
	}
}

func TestTemplateBuild_exceptOnlyPP(t *testing.T) {
	data := `
	{
		"builders": [
			{
				"name": "test1",
				"type": "test-builder"
			},
			{
				"name": "test2",
				"type": "test-builder"
			}
		],

		"post-processors": [
			{
				"type": "test-pp",
				"except": ["test1"],
				"only": ["test1"]
			}
		]
	}
	`

	_, err := ParseTemplate([]byte(data))
	if err == nil {
		t.Fatal("should have error")
	}
}

func TestTemplateBuild_exceptOnlyProv(t *testing.T) {
	data := `
	{
		"builders": [
			{
				"name": "test1",
				"type": "test-builder"
			},
			{
				"name": "test2",
				"type": "test-builder"
			}
		],

		"provisioners": [
			{
				"type": "test-prov",
				"except": ["test1"],
				"only": ["test1"]
			}
		]
	}
	`

	_, err := ParseTemplate([]byte(data))
	if err == nil {
		t.Fatal("should have error")
	}
}

func TestTemplateBuild_exceptPPInvalid(t *testing.T) {
	data := `
	{
		"builders": [
			{
				"name": "test1",
				"type": "test-builder"
			},
			{
				"name": "test2",
				"type": "test-builder"
			}
		],

		"post-processors": [
			{
				"type": "test-pp",
				"except": ["test5"]
			}
		]
	}
	`

	_, err := ParseTemplate([]byte(data))
	if err == nil {
		t.Fatal("should have error")
	}
}

func TestTemplateBuild_exceptPP(t *testing.T) {
	data := `
	{
		"builders": [
			{
				"name": "test1",
				"type": "test-builder"
			},
			{
				"name": "test2",
				"type": "test-builder"
			}
		],

		"post-processors": [
			{
				"type": "test-pp",
				"except": ["test1"]
			}
		]
	}
	`

	template, err := ParseTemplate([]byte(data))
	if err != nil {
		t.Fatalf("err: %s", err)
	}

	// Verify test1 has no post-processors
	build, err := template.Build("test1", testTemplateComponentFinder())
	if err != nil {
		t.Fatalf("err: %s", err)
	}

	cbuild := build.(*coreBuild)
	if len(cbuild.postProcessors) > 0 {
		t.Fatal("should have no postProcessors")
	}

	// Verify test2 has no post-processors
	build, err = template.Build("test2", testTemplateComponentFinder())
	if err != nil {
		t.Fatalf("err: %s", err)
	}

	cbuild = build.(*coreBuild)
	if len(cbuild.postProcessors) != 1 {
		t.Fatalf("invalid: %d", len(cbuild.postProcessors))
	}
}

func TestTemplateBuild_exceptProvInvalid(t *testing.T) {
	data := `
	{
		"builders": [
			{
				"name": "test1",
				"type": "test-builder"
			},
			{
				"name": "test2",
				"type": "test-builder"
			}
		],

		"provisioners": [
			{
				"type": "test-prov",
				"except": ["test5"]
			}
		]
	}
	`

	_, err := ParseTemplate([]byte(data))
	if err == nil {
		t.Fatal("should have error")
	}
}

func TestTemplateBuild_exceptProv(t *testing.T) {
	data := `
	{
		"builders": [
			{
				"name": "test1",
				"type": "test-builder"
			},
			{
				"name": "test2",
				"type": "test-builder"
			}
		],

		"provisioners": [
			{
				"type": "test-prov",
				"except": ["test1"]
			}
		]
	}
	`

	template, err := ParseTemplate([]byte(data))
	if err != nil {
		t.Fatalf("err: %s", err)
	}

	// Verify test1 has no provisioners
	build, err := template.Build("test1", testTemplateComponentFinder())
	if err != nil {
		t.Fatalf("err: %s", err)
	}

	cbuild := build.(*coreBuild)
	if len(cbuild.provisioners) > 0 {
		t.Fatal("should have no provisioners")
	}

	// Verify test2 has no provisioners
	build, err = template.Build("test2", testTemplateComponentFinder())
	if err != nil {
		t.Fatalf("err: %s", err)
	}

	cbuild = build.(*coreBuild)
	if len(cbuild.provisioners) != 1 {
		t.Fatalf("invalid: %d", len(cbuild.provisioners))
	}
}

func TestTemplateBuild_onlyPPInvalid(t *testing.T) {
	data := `
	{
		"builders": [
			{
				"name": "test1",
				"type": "test-builder"
			},
			{
				"name": "test2",
				"type": "test-builder"
			}
		],

		"post-processors": [
			{
				"type": "test-pp",
				"only": ["test5"]
			}
		]
	}
	`

	_, err := ParseTemplate([]byte(data))
	if err == nil {
		t.Fatal("should have error")
	}
}

func TestTemplateBuild_onlyPP(t *testing.T) {
	data := `
	{
		"builders": [
			{
				"name": "test1",
				"type": "test-builder"
			},
			{
				"name": "test2",
				"type": "test-builder"
			}
		],

		"post-processors": [
			{
				"type": "test-pp",
				"only": ["test2"]
			}
		]
	}
	`

	template, err := ParseTemplate([]byte(data))
	if err != nil {
		t.Fatalf("err: %s", err)
	}

	// Verify test1 has no post-processors
	build, err := template.Build("test1", testTemplateComponentFinder())
	if err != nil {
		t.Fatalf("err: %s", err)
	}

	cbuild := build.(*coreBuild)
	if len(cbuild.postProcessors) > 0 {
		t.Fatal("should have no postProcessors")
	}

	// Verify test2 has no post-processors
	build, err = template.Build("test2", testTemplateComponentFinder())
	if err != nil {
		t.Fatalf("err: %s", err)
	}

	cbuild = build.(*coreBuild)
	if len(cbuild.postProcessors) != 1 {
		t.Fatalf("invalid: %d", len(cbuild.postProcessors))
	}
}

func TestTemplateBuild_onlyProvInvalid(t *testing.T) {
	data := `
	{
		"builders": [
			{
				"name": "test1",
				"type": "test-builder"
			},
			{
				"name": "test2",
				"type": "test-builder"
			}
		],

		"provisioners": [
			{
				"type": "test-prov",
				"only": ["test5"]
			}
		]
	}
	`

	_, err := ParseTemplate([]byte(data))
	if err == nil {
		t.Fatal("should have error")
	}
}

func TestTemplateBuild_onlyProv(t *testing.T) {
	data := `
	{
		"builders": [
			{
				"name": "test1",
				"type": "test-builder"
			},
			{
				"name": "test2",
				"type": "test-builder"
			}
		],

		"provisioners": [
			{
				"type": "test-prov",
				"only": ["test2"]
			}
		]
	}
	`

	template, err := ParseTemplate([]byte(data))
	if err != nil {
		t.Fatalf("err: %s", err)
	}

	// Verify test1 has no provisioners
	build, err := template.Build("test1", testTemplateComponentFinder())
	if err != nil {
		t.Fatalf("err: %s", err)
	}

	cbuild := build.(*coreBuild)
	if len(cbuild.provisioners) > 0 {
		t.Fatal("should have no provisioners")
	}

	// Verify test2 has no provisioners
	build, err = template.Build("test2", testTemplateComponentFinder())
	if err != nil {
		t.Fatalf("err: %s", err)
	}

	cbuild = build.(*coreBuild)
	if len(cbuild.provisioners) != 1 {
		t.Fatalf("invalid: %d", len(cbuild.provisioners))
	}
}

func TestTemplate_Build_ProvisionerOverride(t *testing.T) {
	assert := asserts.NewTestingAsserts(t, true)

	data := `
	{
		"builders": [
			{
				"name": "test1",
				"type": "test-builder"
			}
		],

		"provisioners": [
			{
				"type": "test-prov",

				"override": {
					"test1": {}
				}
			}
		]
	}
	`

	template, err := ParseTemplate([]byte(data))
	if err != nil {
		t.Fatalf("err: %s", err)
	}

	RawConfig := template.Provisioners[0].RawConfig
	if RawConfig == nil {
		t.Fatal("missing provisioner raw config")
	}

	expected := map[string]interface{}{
		"type": "test-prov",
	}

	if !reflect.DeepEqual(RawConfig, expected) {
		t.Fatalf("bad raw: %#v", RawConfig)
	}

	builder := testBuilder()
	builderMap := map[string]Builder{
		"test-builder": builder,
	}

	provisioner := &MockProvisioner{}
	provisionerMap := map[string]Provisioner{
		"test-prov": provisioner,
	}

	builderFactory := func(n string) (Builder, error) { return builderMap[n], nil }
	provFactory := func(n string) (Provisioner, error) { return provisionerMap[n], nil }
	components := &ComponentFinder{
		Builder:     builderFactory,
		Provisioner: provFactory,
	}

	// Get the build, verifying we can get it without issue, but also
	// that the proper builder was looked up and used for the build.
	build, err := template.Build("test1", components)
	assert.Nil(err, "should not error")

	coreBuild, ok := build.(*coreBuild)
	assert.True(ok, "should be a core build")
	assert.Equal(len(coreBuild.provisioners), 1, "should have one provisioner")
	assert.Equal(len(coreBuild.provisioners[0].config), 2, "should have two configs on the provisioner")
}

func TestTemplate_Build_ProvisionerOverrideBad(t *testing.T) {
	data := `
	{
		"builders": [
			{
				"name": "test1",
				"type": "test-builder"
			}
		],

		"provisioners": [
			{
				"type": "test-prov",

				"override": {
					"testNope": {}
				}
			}
		]
	}
	`

	_, err := ParseTemplate([]byte(data))
	if err == nil {
		t.Fatal("should have error")
	}
}

func TestTemplateBuild_variables(t *testing.T) {
	data := `
	{
		"variables": {
			"foo": "bar"
		},

		"builders": [
			{
				"name": "test1",
				"type": "test-builder"
			}
		]
	}
	`

	template, err := ParseTemplate([]byte(data))
	if err != nil {
		t.Fatalf("err: %s", err)
	}

	build, err := template.Build("test1", testComponentFinder())
	if err != nil {
		t.Fatalf("err: %s", err)
	}

	coreBuild, ok := build.(*coreBuild)
	if !ok {
		t.Fatalf("couldn't convert!")
	}

	if len(coreBuild.variables) != 1 {
		t.Fatalf("bad vars: %#v", coreBuild.variables)
	}
}
