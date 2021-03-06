package docker

import (
	"flag"
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"testing"

	"github.com/amane3/goreleaser/internal/artifact"
	"github.com/amane3/goreleaser/internal/pipe"
	"github.com/amane3/goreleaser/internal/testlib"
	"github.com/amane3/goreleaser/pkg/config"
	"github.com/amane3/goreleaser/pkg/context"
	"github.com/stretchr/testify/require"
)

var it = flag.Bool("it", false, "push images to docker hub")
var registry = "localhost:5000/"
var altRegistry = "localhost:5050/"

func TestMain(m *testing.M) {
	flag.Parse()
	if *it {
		registry = "docker.io/"
	}
	os.Exit(m.Run())
}

func start(t *testing.T) {
	if *it {
		return
	}
	if out, err := exec.Command(
		"docker", "run", "-d", "-p", "5000:5000", "--name", "registry", "registry:2",
	).CombinedOutput(); err != nil {
		t.Log("failed to start docker registry", string(out), err)
		t.FailNow()
	}
	if out, err := exec.Command(
		"docker", "run", "-d", "-p", "5050:5000", "--name", "alt_registry", "registry:2",
	).CombinedOutput(); err != nil {
		t.Log("failed to start alternate docker registry", string(out), err)
		t.FailNow()
	}
}

func killAndRm(t *testing.T) {
	if *it {
		return
	}
	t.Log("killing registry")
	_ = exec.Command("docker", "kill", "registry").Run()
	_ = exec.Command("docker", "rm", "registry").Run()
	_ = exec.Command("docker", "kill", "alt_registry").Run()
	_ = exec.Command("docker", "rm", "alt_registry").Run()
}

// TODO: this test is too big... split in smaller tests? Mainly the manifest ones...
func TestRunPipe(t *testing.T) {
	type errChecker func(*testing.T, error)
	var shouldErr = func(msg string) errChecker {
		return func(t *testing.T, err error) {
			require.Error(t, err)
			require.Contains(t, err.Error(), msg)
		}
	}
	var shouldNotErr = func(t *testing.T, err error) {
		require.NoError(t, err)
	}
	type imageLabelFinder func(*testing.T, int)
	var shouldFindImagesWithLabels = func(image string, filters ...string) func(*testing.T, int) {
		return func(t *testing.T, count int) {
			for _, filter := range filters {
				output, err := exec.Command(
					"docker", "images", "-q", "*/"+image,
					"--filter", filter,
				).CombinedOutput()
				require.NoError(t, err)
				lines := strings.Split(strings.TrimSpace(string(output)), "\n")
				require.Equal(t, count, len(lines))
			}
		}

	}
	var noLabels = func(t *testing.T, count int) {}

	var table = map[string]struct {
		dockers             []config.Docker
		manifests           []config.DockerManifest
		env                 map[string]string
		expect              []string
		assertImageLabels   imageLabelFinder
		assertError         errChecker
		pubAssertError      errChecker
		manifestAssertError errChecker
	}{
		"multiarch": {
			dockers: []config.Docker{
				{
					ImageTemplates:     []string{registry + "goreleaser/test_multiarch:test-amd64"},
					Goos:               "linux",
					Goarch:             "amd64",
					Dockerfile:         "testdata/Dockerfile.arch",
					Binaries:           []string{"mybin"},
					BuildFlagTemplates: []string{"--build-arg", "ARCH=amd64"},
				},
				{
					ImageTemplates:     []string{registry + "goreleaser/test_multiarch:test-arm64v8"},
					Goos:               "linux",
					Goarch:             "arm64",
					Dockerfile:         "testdata/Dockerfile.arch",
					Binaries:           []string{"mybin"},
					BuildFlagTemplates: []string{"--build-arg", "ARCH=arm64v8"},
				},
			},
			manifests: []config.DockerManifest{
				{
					// XXX: fails if :latest https://github.com/docker/distribution/issues/3100
					NameTemplate: registry + "goreleaser/test_multiarch:test",
					ImageTemplates: []string{
						registry + "goreleaser/test_multiarch:test-amd64",
						registry + "goreleaser/test_multiarch:test-arm64v8",
					},
					CreateFlags: []string{"--insecure"},
					PushFlags:   []string{"--insecure"},
				},
			},
			expect: []string{
				registry + "goreleaser/test_multiarch:test-amd64",
				registry + "goreleaser/test_multiarch:test-arm64v8",
			},
			assertError:         shouldNotErr,
			pubAssertError:      shouldNotErr,
			manifestAssertError: shouldNotErr,
			assertImageLabels:   noLabels,
		},
		"multiarch image not found": {
			dockers: []config.Docker{
				{
					ImageTemplates:     []string{registry + "goreleaser/test_multiarch_fail:latest-arm64v8"},
					Goos:               "linux",
					Goarch:             "arm64",
					Dockerfile:         "testdata/Dockerfile.arch",
					Binaries:           []string{"mybin"},
					BuildFlagTemplates: []string{"--build-arg", "ARCH=arm64v8"},
				},
			},
			manifests: []config.DockerManifest{
				{
					NameTemplate:   registry + "goreleaser/test_multiarch_fail:test",
					ImageTemplates: []string{registry + "goreleaser/test_multiarch_fail:latest-amd64"},
					CreateFlags:    []string{"--insecure"},
					PushFlags:      []string{"--insecure"},
				},
			},
			expect:              []string{registry + "goreleaser/test_multiarch_fail:latest-arm64v8"},
			assertError:         shouldNotErr,
			pubAssertError:      shouldNotErr,
			manifestAssertError: shouldErr("failed to create docker manifest: localhost:5000/goreleaser/test_multiarch_fail:test"),
			assertImageLabels:   noLabels,
		},
		"multiarch manifest template error": {
			dockers: []config.Docker{
				{
					ImageTemplates: []string{registry + "goreleaser/test_multiarch_manifest_tmpl_error"},
					Goos:           "linux",
					Goarch:         "arm64",
					Dockerfile:     "testdata/Dockerfile",
					Binaries:       []string{"mybin"},
				},
			},
			manifests: []config.DockerManifest{
				{
					NameTemplate:   registry + "goreleaser/test_multiarch_manifest_tmpl_error:{{ .Goos }",
					ImageTemplates: []string{registry + "goreleaser/test_multiarch_manifest_tmpl_error"},
				},
			},
			expect:              []string{registry + "goreleaser/test_multiarch_manifest_tmpl_error"},
			assertError:         shouldNotErr,
			pubAssertError:      shouldNotErr,
			manifestAssertError: shouldErr(`template: tmpl:1: unexpected "}" in operand`),
			assertImageLabels:   noLabels,
		},
		"multiarch image template error": {
			dockers: []config.Docker{
				{
					ImageTemplates: []string{registry + "goreleaser/test_multiarch_img_tmpl_error"},
					Goos:           "linux",
					Goarch:         "arm64",
					Dockerfile:     "testdata/Dockerfile",
					Binaries:       []string{"mybin"},
				},
			},
			manifests: []config.DockerManifest{
				{
					NameTemplate:   registry + "goreleaser/test_multiarch_img_tmpl_error",
					ImageTemplates: []string{registry + "goreleaser/test_multiarch_img_tmpl_error:{{ .Goos }"},
				},
			},
			expect:              []string{registry + "goreleaser/test_multiarch_img_tmpl_error"},
			assertError:         shouldNotErr,
			pubAssertError:      shouldNotErr,
			manifestAssertError: shouldErr(`template: tmpl:1: unexpected "}" in operand`),
			assertImageLabels:   noLabels,
		},
		"multiarch missing manifest name": {
			dockers: []config.Docker{
				{
					ImageTemplates: []string{registry + "goreleaser/test_multiarch_no_mainifest_name"},
					Goos:           "linux",
					Goarch:         "arm64",
					Dockerfile:     "testdata/Dockerfile",
					Binaries:       []string{"mybin"},
				},
			},
			manifests: []config.DockerManifest{
				{
					NameTemplate:   "  ",
					ImageTemplates: []string{registry + "goreleaser/test_multiarch_no_mainifest_name"},
				},
			},
			expect:              []string{registry + "goreleaser/test_multiarch_no_mainifest_name"},
			assertError:         shouldNotErr,
			pubAssertError:      shouldNotErr,
			manifestAssertError: testlib.AssertSkipped,
			assertImageLabels:   noLabels,
		},
		"multiarch missing images": {
			dockers: []config.Docker{
				{
					ImageTemplates: []string{registry + "goreleaser/test_multiarch_no_mainifest_images"},
					Dockerfile:     "testdata/Dockerfile",
					Goos:           "linux",
					Goarch:         "arm64",
					Binaries:       []string{"mybin"},
				},
			},
			manifests: []config.DockerManifest{
				{
					NameTemplate:   "ignored",
					ImageTemplates: []string{" ", "   ", ""},
				},
			},
			expect:              []string{registry + "goreleaser/test_multiarch_no_mainifest_images"},
			assertError:         shouldNotErr,
			pubAssertError:      shouldNotErr,
			manifestAssertError: testlib.AssertSkipped,
			assertImageLabels:   noLabels,
		},
		"valid": {
			env: map[string]string{
				"FOO": "123",
			},
			dockers: []config.Docker{
				{
					ImageTemplates: []string{
						registry + "goreleaser/test_run_pipe:{{.Tag}}-{{.Env.FOO}}",
						registry + "goreleaser/test_run_pipe:v{{.Major}}",
						registry + "goreleaser/test_run_pipe:v{{.Major}}.{{.Minor}}",
						registry + "goreleaser/test_run_pipe:commit-{{.Commit}}",
						registry + "goreleaser/test_run_pipe:latest",
						altRegistry + "goreleaser/test_run_pipe:{{.Tag}}-{{.Env.FOO}}",
						altRegistry + "goreleaser/test_run_pipe:v{{.Major}}",
						altRegistry + "goreleaser/test_run_pipe:v{{.Major}}.{{.Minor}}",
						altRegistry + "goreleaser/test_run_pipe:commit-{{.Commit}}",
						altRegistry + "goreleaser/test_run_pipe:latest",
					},
					Goos:       "linux",
					Goarch:     "amd64",
					Dockerfile: "testdata/Dockerfile",
					Binaries:   []string{"mybin"},
					BuildFlagTemplates: []string{
						"--label=org.label-schema.schema-version=1.0",
						"--label=org.label-schema.version={{.Version}}",
						"--label=org.label-schema.vcs-ref={{.Commit}}",
						"--label=org.label-schema.name={{.ProjectName}}",
						"--build-arg=FRED={{.Tag}}",
					},
					Files: []string{
						"testdata/extra_file.txt",
					},
				},
			},
			expect: []string{
				registry + "goreleaser/test_run_pipe:v1.0.0-123",
				registry + "goreleaser/test_run_pipe:v1",
				registry + "goreleaser/test_run_pipe:v1.0",
				registry + "goreleaser/test_run_pipe:commit-a1b2c3d4",
				registry + "goreleaser/test_run_pipe:latest",
				altRegistry + "goreleaser/test_run_pipe:v1.0.0-123",
				altRegistry + "goreleaser/test_run_pipe:v1",
				altRegistry + "goreleaser/test_run_pipe:v1.0",
				altRegistry + "goreleaser/test_run_pipe:commit-a1b2c3d4",
				altRegistry + "goreleaser/test_run_pipe:latest",
			},
			assertImageLabels: shouldFindImagesWithLabels(
				"goreleaser/test_run_pipe",
				"label=org.label-schema.schema-version=1.0",
				"label=org.label-schema.version=1.0.0",
				"label=org.label-schema.vcs-ref=a1b2c3d4",
				"label=org.label-schema.name=mybin",
			),
			assertError:         shouldNotErr,
			pubAssertError:      shouldNotErr,
			manifestAssertError: shouldNotErr,
		},
		"valid-with-builds": {
			dockers: []config.Docker{
				{
					ImageTemplates: []string{
						registry + "goreleaser/test_run_pipe_build:latest",
					},
					Goos:       "linux",
					Goarch:     "amd64",
					Dockerfile: "testdata/Dockerfile",
					Binaries:   []string{"mybin"},
					Builds:     []string{"mybin"},
				},
			},
			expect: []string{
				registry + "goreleaser/test_run_pipe_build:latest",
			},
			assertImageLabels:   noLabels,
			assertError:         shouldNotErr,
			pubAssertError:      shouldNotErr,
			manifestAssertError: shouldNotErr,
		},
		"multiple images with same extra file": {
			dockers: []config.Docker{
				{
					ImageTemplates: []string{
						registry + "goreleaser/multiplefiles1:latest",
					},
					Goos:       "linux",
					Goarch:     "amd64",
					Dockerfile: "testdata/Dockerfile",
					Binaries:   []string{"mybin"},
					Files:      []string{"testdata/extra_file.txt"},
				},
				{
					ImageTemplates: []string{
						registry + "goreleaser/multiplefiles2:latest",
					},
					Goos:       "linux",
					Goarch:     "amd64",
					Dockerfile: "testdata/Dockerfile",
					Binaries:   []string{"mybin"},
					Files:      []string{"testdata/extra_file.txt"},
				},
			},
			expect: []string{
				registry + "goreleaser/multiplefiles1:latest",
				registry + "goreleaser/multiplefiles2:latest",
			},
			assertImageLabels:   noLabels,
			assertError:         shouldNotErr,
			pubAssertError:      shouldNotErr,
			manifestAssertError: shouldNotErr,
		},
		"multiple images with same dockerfile": {
			dockers: []config.Docker{
				{
					ImageTemplates: []string{
						registry + "goreleaser/test_run_pipe:latest",
					},
					Goos:       "linux",
					Goarch:     "amd64",
					Dockerfile: "testdata/Dockerfile",
					Binaries:   []string{"mybin"},
				},
				{
					ImageTemplates: []string{
						registry + "goreleaser/test_run_pipe2:latest",
					},
					Goos:       "linux",
					Goarch:     "amd64",
					Dockerfile: "testdata/Dockerfile",
					Binaries:   []string{"mybin"},
				},
			},
			assertImageLabels: noLabels,
			expect: []string{
				registry + "goreleaser/test_run_pipe:latest",
				registry + "goreleaser/test_run_pipe2:latest",
			},
			assertError:         shouldNotErr,
			pubAssertError:      shouldNotErr,
			manifestAssertError: shouldNotErr,
		},
		"valid_skip_push": {
			dockers: []config.Docker{
				{
					ImageTemplates: []string{
						registry + "goreleaser/test_run_pipe:latest",
					},
					Goos:       "linux",
					Goarch:     "amd64",
					Dockerfile: "testdata/Dockerfile",
					Binaries:   []string{"mybin"},
					SkipPush:   "true",
				},
			},
			expect: []string{
				registry + "goreleaser/test_run_pipe:latest",
			},
			assertImageLabels: noLabels,
			assertError:       testlib.AssertSkipped,
		},
		"one_img_error_with_skip_push": {
			dockers: []config.Docker{
				{
					ImageTemplates: []string{
						registry + "goreleaser/one_img_error_with_skip_push:true",
					},
					Goos:       "linux",
					Goarch:     "amd64",
					Dockerfile: "testdata/Dockerfile.true",
					Binaries:   []string{"mybin"},
					SkipPush:   "true",
				},
				{
					ImageTemplates: []string{
						registry + "goreleaser/one_img_error_with_skip_push:false",
					},
					Goos:       "linux",
					Goarch:     "amd64",
					Dockerfile: "testdata/Dockerfile.false",
					Binaries:   []string{"mybin"},
					SkipPush:   "true",
				},
			},
			expect: []string{
				registry + "goreleaser/one_img_error_with_skip_push:true",
			},
			assertImageLabels: noLabels,
			assertError:       shouldErr("failed to build docker image"),
		},
		"valid_no_latest": {
			dockers: []config.Docker{
				{
					ImageTemplates: []string{
						registry + "goreleaser/test_run_pipe:{{.Version}}",
					},
					Goos:       "linux",
					Goarch:     "amd64",
					Dockerfile: "testdata/Dockerfile",
					Binaries:   []string{"mybin"},
				},
			},
			expect: []string{
				registry + "goreleaser/test_run_pipe:1.0.0",
			},
			assertImageLabels:   noLabels,
			assertError:         shouldNotErr,
			pubAssertError:      shouldNotErr,
			manifestAssertError: shouldNotErr,
		},
		"valid build args": {
			dockers: []config.Docker{
				{
					ImageTemplates: []string{
						registry + "goreleaser/test_build_args:latest",
					},
					Goos:       "linux",
					Goarch:     "amd64",
					Dockerfile: "testdata/Dockerfile",
					Binaries:   []string{"mybin"},
					BuildFlagTemplates: []string{
						"--label=foo=bar",
					},
				},
			},
			expect: []string{
				registry + "goreleaser/test_build_args:latest",
			},
			assertImageLabels:   noLabels,
			assertError:         shouldNotErr,
			pubAssertError:      shouldNotErr,
			manifestAssertError: shouldNotErr,
		},
		"bad build args": {
			dockers: []config.Docker{
				{
					ImageTemplates: []string{
						registry + "goreleaser/test_build_args:latest",
					},
					Goos:       "linux",
					Goarch:     "amd64",
					Dockerfile: "testdata/Dockerfile",
					Binaries:   []string{"mybin"},
					BuildFlagTemplates: []string{
						"--bad-flag",
					},
				},
			},
			assertImageLabels: noLabels,
			assertError:       shouldErr("unknown flag: --bad-flag"),
		},
		"bad_dockerfile": {
			dockers: []config.Docker{
				{
					ImageTemplates: []string{
						registry + "goreleaser/bad_dockerfile:latest",
					},
					Goos:       "linux",
					Goarch:     "amd64",
					Dockerfile: "testdata/Dockerfile.bad",
					Binaries:   []string{"mybin"},
				},
			},
			assertImageLabels: noLabels,
			assertError:       shouldErr("pull access denied for nope, repository does not exist"),
		},
		"tag_template_error": {
			dockers: []config.Docker{
				{
					ImageTemplates: []string{
						registry + "goreleaser/test_run_pipe:{{.Tag}",
					},
					Goos:       "linux",
					Goarch:     "amd64",
					Dockerfile: "testdata/Dockerfile",
					Binaries:   []string{"mybin"},
				},
			},
			assertImageLabels: noLabels,
			assertError:       shouldErr(`template: tmpl:1: unexpected "}" in operand`),
		},
		"build_flag_template_error": {
			dockers: []config.Docker{
				{
					ImageTemplates: []string{
						registry + "goreleaser/test_run_pipe:latest",
					},
					Goos:       "linux",
					Goarch:     "amd64",
					Dockerfile: "testdata/Dockerfile",
					Binaries:   []string{"mybin"},
					BuildFlagTemplates: []string{
						"--label=tag={{.Tag}",
					},
				},
			},
			assertImageLabels: noLabels,
			assertError:       shouldErr(`template: tmpl:1: unexpected "}" in operand`),
		},
		"missing_env_on_tag_template": {
			dockers: []config.Docker{
				{
					ImageTemplates: []string{
						registry + "goreleaser/test_run_pipe:{{.Env.NOPE}}",
					},
					Goos:       "linux",
					Goarch:     "amd64",
					Dockerfile: "testdata/Dockerfile",
					Binaries:   []string{"mybin"},
				},
			},
			assertImageLabels: noLabels,
			assertError:       shouldErr(`template: tmpl:1:46: executing "tmpl" at <.Env.NOPE>: map has no entry for key "NOPE"`),
		},
		"missing_env_on_build_flag_template": {
			dockers: []config.Docker{
				{
					ImageTemplates: []string{
						registry + "goreleaser/test_run_pipe:latest",
					},
					Goos:       "linux",
					Goarch:     "amd64",
					Dockerfile: "testdata/Dockerfile",
					Binaries:   []string{"mybin"},
					BuildFlagTemplates: []string{
						"--label=nope={{.Env.NOPE}}",
					},
				},
			},
			assertImageLabels: noLabels,
			assertError:       shouldErr(`template: tmpl:1:19: executing "tmpl" at <.Env.NOPE>: map has no entry for key "NOPE"`),
		},
		"image_has_projectname_template_variable": {
			dockers: []config.Docker{
				{
					ImageTemplates: []string{
						registry + "goreleaser/{{.ProjectName}}:{{.Tag}}-{{.Env.FOO}}",
						registry + "goreleaser/{{.ProjectName}}:latest",
					},
					Goos:       "linux",
					Goarch:     "amd64",
					Dockerfile: "testdata/Dockerfile",
					Binaries:   []string{"mybin"},
					SkipPush:   "true",
				},
			},
			env: map[string]string{
				"FOO": "123",
			},
			expect: []string{
				registry + "goreleaser/mybin:v1.0.0-123",
				registry + "goreleaser/mybin:latest",
			},
			assertImageLabels: noLabels,
			assertError:       testlib.AssertSkipped,
		},
		"no_permissions": {
			dockers: []config.Docker{
				{
					ImageTemplates: []string{"docker.io/nope:latest"},
					Goos:           "linux",
					Goarch:         "amd64",
					Binaries:       []string{"mybin"},
					Dockerfile:     "testdata/Dockerfile",
				},
			},
			expect: []string{
				"docker.io/nope:latest",
			},
			assertImageLabels:   noLabels,
			assertError:         shouldNotErr,
			pubAssertError:      shouldErr(`requested access to the resource is denied`),
			manifestAssertError: shouldNotErr,
		},
		"dockerfile_doesnt_exist": {
			dockers: []config.Docker{
				{
					ImageTemplates: []string{"whatever:latest"},
					Goos:           "linux",
					Goarch:         "amd64",
					Binaries:       []string{"mybin"},
					Dockerfile:     "testdata/Dockerfilezzz",
				},
			},
			assertImageLabels: noLabels,
			assertError:       shouldErr(`failed to link dockerfile`),
		},
		"extra_file_doesnt_exist": {
			dockers: []config.Docker{
				{
					ImageTemplates: []string{"whatever:latest"},
					Goos:           "linux",
					Goarch:         "amd64",
					Binaries:       []string{"mybin"},
					Dockerfile:     "testdata/Dockerfile",
					Files: []string{
						"testdata/nope.txt",
					},
				},
			},
			assertImageLabels: noLabels,
			assertError:       shouldErr(`failed to link extra file 'testdata/nope.txt'`),
		},
		"no_matching_binaries": {
			dockers: []config.Docker{
				{
					ImageTemplates: []string{"whatever:latest"},
					Goos:           "darwin",
					Goarch:         "amd64",
					Binaries:       []string{"mybinnnn"},
					Dockerfile:     "testdata/Dockerfile",
				},
			},
			assertImageLabels: noLabels,
			assertError:       shouldErr(`0 binaries match docker definition: [mybinnnn]: darwin_amd64_, should be 1`),
		},
		"multiple_binaries": {
			dockers: []config.Docker{
				{
					ImageTemplates: []string{registry + "goreleaser/multiple:latest"},
					Goos:           "darwin",
					Goarch:         "amd64",
					Binaries:       []string{"mybin", "anotherbin"},
					Dockerfile:     "testdata/Dockerfile.multiple",
				},
			},
			assertImageLabels:   noLabels,
			assertError:         shouldNotErr,
			pubAssertError:      shouldNotErr,
			manifestAssertError: shouldNotErr,
			expect: []string{
				registry + "goreleaser/multiple:latest",
			},
		},
		// TODO: add a test case for multiple matching binaries for the same name
		"templated_binaries": {
			env: map[string]string{
				"BIN_NAME": "mybin",
			},
			dockers: []config.Docker{
				{
					ImageTemplates: []string{registry + "goreleaser/templatedbins:latest"},
					Goos:           "darwin",
					Goarch:         "amd64",
					Binaries:       []string{"{{.Env.BIN_NAME}}"},
					Dockerfile:     "testdata/Dockerfile",
				},
			},
			assertImageLabels:   noLabels,
			assertError:         shouldNotErr,
			pubAssertError:      shouldNotErr,
			manifestAssertError: shouldNotErr,
			expect: []string{
				registry + "goreleaser/templatedbins:latest",
			},
		},
		"binaries_template_error": {
			dockers: []config.Docker{
				{
					ImageTemplates: []string{
						registry + "goreleaser/binaries_template_error:latest",
					},
					Goos:       "linux",
					Goarch:     "amd64",
					Dockerfile: "testdata/Dockerfile",
					Binaries:   []string{"{{.Env.BAR}"},
				},
			},
			assertImageLabels: noLabels,
			assertError:       shouldErr(`template: tmpl:1: unexpected "}" in operand`),
		},
	}

	killAndRm(t)
	start(t)
	defer killAndRm(t)

	for name, docker := range table {
		t.Run(name, func(tt *testing.T) {
			var folder = t.TempDir()
			var dist = filepath.Join(folder, "dist")
			require.NoError(tt, os.Mkdir(dist, 0755))
			require.NoError(tt, os.Mkdir(filepath.Join(dist, "mybin"), 0755))
			_, err := os.Create(filepath.Join(dist, "mybin", "mybin"))
			require.NoError(tt, err)
			_, err = os.Create(filepath.Join(dist, "mybin", "anotherbin"))
			require.NoError(tt, err)

			var ctx = context.New(config.Project{
				ProjectName:     "mybin",
				Dist:            dist,
				Dockers:         docker.dockers,
				DockerManifests: docker.manifests,
			})
			ctx.Parallelism = 1
			ctx.Env = docker.env
			ctx.Version = "1.0.0"
			ctx.Git = context.GitInfo{
				CurrentTag: "v1.0.0",
				Commit:     "a1b2c3d4",
			}
			ctx.Semver = context.Semver{
				Major: 1,
				Minor: 0,
				Patch: 0,
			}
			for _, os := range []string{"linux", "darwin"} {
				for _, arch := range []string{"amd64", "386", "arm64"} {
					for _, bin := range []string{"mybin", "anotherbin"} {
						ctx.Artifacts.Add(&artifact.Artifact{
							Name:   bin,
							Path:   filepath.Join(dist, "mybin", bin),
							Goarch: arch,
							Goos:   os,
							Type:   artifact.Binary,
							Extra: map[string]interface{}{
								"Binary": bin,
								"ID":     bin,
							},
						})
					}
				}
			}

			// this might fail as the image doesnt exist yet, so lets ignore the error
			for _, img := range docker.expect {
				_ = exec.Command("docker", "rmi", img).Run()
			}

			err = Pipe{}.Run(ctx)
			docker.assertError(tt, err)
			if err == nil {
				docker.pubAssertError(tt, Pipe{}.Publish(ctx))
				docker.manifestAssertError(tt, ManifestPipe{}.Publish(ctx))
			}

			for _, d := range docker.dockers {
				docker.assertImageLabels(tt, len(d.ImageTemplates))
			}

			// this might should not fail as the image should have been created when
			// the step ran
			for _, img := range docker.expect {
				tt.Log("removing docker image", img)
				require.NoError(tt, exec.Command("docker", "rmi", img).Run(), "could not delete image %s", img)
			}

		})
	}
}

func TestBuildCommand(t *testing.T) {
	images := []string{"goreleaser/test_build_flag", "goreleaser/test_multiple_tags"}
	tests := []struct {
		name   string
		flags  []string
		expect []string
	}{
		{
			name:   "no flags",
			flags:  []string{},
			expect: []string{"build", ".", "-t", images[0], "-t", images[1]},
		},
		{
			name:   "single flag",
			flags:  []string{"--label=foo"},
			expect: []string{"build", ".", "-t", images[0], "-t", images[1], "--label=foo"},
		},
		{
			name:   "multiple flags",
			flags:  []string{"--label=foo", "--build-arg=bar=baz"},
			expect: []string{"build", ".", "-t", images[0], "-t", images[1], "--label=foo", "--build-arg=bar=baz"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			command := buildCommand(images, tt.flags)
			require.Equal(t, tt.expect, command)
		})
	}
}

func TestDescription(t *testing.T) {
	require.NotEmpty(t, Pipe{}.String())
}

func TestNoDockers(t *testing.T) {
	require.True(t, pipe.IsSkip(Pipe{}.Run(context.New(config.Project{}))))
}

func TestNoDockerWithoutImageName(t *testing.T) {
	require.True(t, pipe.IsSkip(Pipe{}.Run(context.New(config.Project{
		Dockers: []config.Docker{
			{
				Goos: "linux",
			},
		},
	}))))
}

func TestDockerNotInPath(t *testing.T) {
	var path = os.Getenv("PATH")
	defer func() {
		require.NoError(t, os.Setenv("PATH", path))
	}()
	require.NoError(t, os.Setenv("PATH", ""))
	var ctx = &context.Context{
		Version: "1.0.0",
		Config: config.Project{
			Dockers: []config.Docker{
				{
					ImageTemplates: []string{"a/b"},
				},
			},
		},
	}
	require.EqualError(t, Pipe{}.Run(ctx), ErrNoDocker.Error())
}

func TestDefault(t *testing.T) {
	var ctx = &context.Context{
		Config: config.Project{
			Builds: []config.Build{
				{
					Binary: "foo",
				},
			},
			Dockers: []config.Docker{
				{},
			},
		},
	}
	require.NoError(t, Pipe{}.Default(ctx))
	require.Len(t, ctx.Config.Dockers, 1)
	var docker = ctx.Config.Dockers[0]
	require.Equal(t, "linux", docker.Goos)
	require.Equal(t, "amd64", docker.Goarch)
	require.Equal(t, []string{ctx.Config.Builds[0].Binary}, docker.Binaries)
	require.Empty(t, docker.Builds)
}

func TestDefaultDockerfile(t *testing.T) {
	var ctx = &context.Context{
		Config: config.Project{
			Builds: []config.Build{
				{},
			},
			Dockers: []config.Docker{
				{},
				{},
			},
		},
	}
	require.NoError(t, Pipe{}.Default(ctx))
	require.Len(t, ctx.Config.Dockers, 2)
	require.Equal(t, "Dockerfile", ctx.Config.Dockers[0].Dockerfile)
	require.Equal(t, "Dockerfile", ctx.Config.Dockers[1].Dockerfile)
}

func TestDefaultBinaries(t *testing.T) {
	var ctx = &context.Context{
		Config: config.Project{
			Builds: []config.Build{
				{
					ID: "foo",
				},
			},
			Dockers: []config.Docker{
				{
					Binaries: []string{"foo"},
				},
			},
		},
	}
	require.NoError(t, Pipe{}.Default(ctx))
	require.Len(t, ctx.Config.Dockers, 1)
	var docker = ctx.Config.Dockers[0]
	require.Equal(t, "linux", docker.Goos)
	require.Equal(t, "amd64", docker.Goarch)
	require.Equal(t, []string{"foo"}, docker.Binaries)
}

func TestDefaultNoDockers(t *testing.T) {
	var ctx = &context.Context{
		Config: config.Project{
			Dockers: []config.Docker{},
		},
	}
	require.NoError(t, Pipe{}.Default(ctx))
	require.Empty(t, ctx.Config.Dockers)
}

func TestDefaultFilesDot(t *testing.T) {
	var ctx = &context.Context{
		Config: config.Project{
			Dist: "/tmp/distt",
			Dockers: []config.Docker{
				{
					Files: []string{"./lala", "./lolsob", "."},
				},
			},
		},
	}
	require.EqualError(t, Pipe{}.Default(ctx), `invalid docker.files: can't be . or inside dist folder: .`)
}

func TestDefaultFilesDis(t *testing.T) {
	var ctx = &context.Context{
		Config: config.Project{
			Dist: "/tmp/dist",
			Dockers: []config.Docker{
				{
					Files: []string{"./fooo", "/tmp/dist/asdasd/asd", "./bar"},
				},
			},
		},
	}
	require.EqualError(t, Pipe{}.Default(ctx), `invalid docker.files: can't be . or inside dist folder: /tmp/dist/asdasd/asd`)
}

func TestDefaultSet(t *testing.T) {
	var ctx = &context.Context{
		Config: config.Project{
			Dockers: []config.Docker{
				{
					Builds:     []string{"foo"},
					Goos:       "windows",
					Goarch:     "i386",
					Binaries:   []string{"bar"},
					Dockerfile: "Dockerfile.foo",
				},
			},
		},
	}
	require.NoError(t, Pipe{}.Default(ctx))
	require.Len(t, ctx.Config.Dockers, 1)
	var docker = ctx.Config.Dockers[0]
	require.Equal(t, "windows", docker.Goos)
	require.Equal(t, "i386", docker.Goarch)
	require.Equal(t, "bar", docker.Binaries[0])
	require.Equal(t, "foo", docker.Builds[0])
	require.Equal(t, "Dockerfile.foo", docker.Dockerfile)
}

func Test_processImageTemplates(t *testing.T) {
	var ctx = &context.Context{
		Config: config.Project{
			Builds: []config.Build{
				{
					ID: "default",
				},
			},
			Dockers: []config.Docker{
				{
					Binaries:   []string{"foo"},
					Dockerfile: "Dockerfile.foo",
					ImageTemplates: []string{
						"user/image:{{.Tag}}",
						"gcr.io/image:{{.Tag}}-{{.Env.FOO}}",
						"gcr.io/image:v{{.Major}}.{{.Minor}}",
					},
					SkipPush: "true",
				},
			},
		},
	}
	ctx.SkipPublish = true
	ctx.Env = map[string]string{
		"FOO": "123",
	}
	ctx.Version = "1.0.0"
	ctx.Git = context.GitInfo{
		CurrentTag: "v1.0.0",
		Commit:     "a1b2c3d4",
	}
	ctx.Semver = context.Semver{
		Major: 1,
		Minor: 0,
		Patch: 0,
	}

	require.NoError(t, Pipe{}.Default(ctx))
	require.Len(t, ctx.Config.Dockers, 1)

	docker := ctx.Config.Dockers[0]
	require.Equal(t, "Dockerfile.foo", docker.Dockerfile)

	images, err := processImageTemplates(ctx, docker)
	require.NoError(t, err)
	require.Equal(t, []string{
		"user/image:v1.0.0",
		"gcr.io/image:v1.0.0-123",
		"gcr.io/image:v1.0",
	}, images)
}

func TestLinkFile(t *testing.T) {
	src, err := ioutil.TempFile(t.TempDir(), "src")
	require.NoError(t, err)
	require.NoError(t, src.Close())
	dst := filepath.Join(filepath.Dir(src.Name()), "dst")
	t.Cleanup(func() {
		os.Remove(src.Name())
		os.Remove(dst)
	})
	fmt.Println("src:", src.Name())
	fmt.Println("dst:", dst)
	require.NoError(t, ioutil.WriteFile(src.Name(), []byte("foo"), 0644))
	require.NoError(t, link(src.Name(), dst))
	require.Equal(t, inode(src.Name()), inode(dst))
}

func TestLinkDirectory(t *testing.T) {
	var srcDir = t.TempDir()
	var dstDir = t.TempDir()
	const testFile = "test"
	require.NoError(t, ioutil.WriteFile(filepath.Join(srcDir, testFile), []byte("foo"), 0644))
	require.NoError(t, link(srcDir, dstDir))
	require.Equal(t, inode(filepath.Join(srcDir, testFile)), inode(filepath.Join(dstDir, testFile)))
}

func TestLinkTwoLevelDirectory(t *testing.T) {
	var srcDir = t.TempDir()
	var dstDir = t.TempDir()
	var srcLevel2 = filepath.Join(srcDir, "level2")
	const testFile = "test"

	require.NoError(t, os.Mkdir(srcLevel2, 0755))
	require.NoError(t, ioutil.WriteFile(filepath.Join(srcDir, testFile), []byte("foo"), 0644))
	require.NoError(t, ioutil.WriteFile(filepath.Join(srcLevel2, testFile), []byte("foo"), 0644))

	require.NoError(t, link(srcDir, dstDir))

	require.Equal(t, inode(filepath.Join(srcDir, testFile)), inode(filepath.Join(dstDir, testFile)))
	require.Equal(t, inode(filepath.Join(srcLevel2, testFile)), inode(filepath.Join(dstDir, "level2", testFile)))
}

func inode(file string) uint64 {
	fileInfo, err := os.Stat(file)
	if err != nil {
		return 0
	}
	stat := fileInfo.Sys().(*syscall.Stat_t)
	return stat.Ino
}
