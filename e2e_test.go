package videoworkflows

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"testing"
	"time"

	"golang.org/x/mod/modfile"

	"github.com/docker/docker/api/types/build"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/network"
	"github.com/testcontainers/testcontainers-go/wait"
)

func readModfile(t *testing.T) *modfile.File {
	data, err := os.ReadFile("go.mod")
	if err != nil {
		t.Fatalf("failed to read go.mod: %v", err)
	}

	modFile, err := modfile.Parse("go.mod", data, nil)
	if err != nil {
		t.Fatalf("failed to parse go.mod: %v", err)
	}

	return modFile
}

func transcodeTag(t *testing.T) string {
	modFile := readModfile(t)

	for _, req := range modFile.Require {
		if req.Mod.Path == "github.com/krelinga/video-transcoder" {
			return req.Mod.Version
		}
	}

	t.Fatal("github.com/krelinga/video-transcoder not found in go.mod")
	return ""
}

func videoInfoTag(t *testing.T) string {
	modFile := readModfile(t)

	for _, req := range modFile.Require {
		if req.Mod.Path == "github.com/krelinga/video-info" {
			return req.Mod.Version
		}
	}

	t.Fatal("github.com/krelinga/video-info not found in go.mod")
	return ""
}

func TestEnd2End(t *testing.T) {
	t.Logf("Using video-transcoder version: %s", transcodeTag(t))
	t.Logf("Using video-info version: %s", videoInfoTag(t))

	// Create temp directory for media files
	tempDir, err := os.MkdirTemp("", "transcode-e2e-*")
	if err != nil {
		t.Fatalf("failed to create temp directory: %v", err)
	}

	// Copy test file to temp directory
	srcFile := "testdata/testdata_sample_640x360.mkv"
	dstFile := filepath.Join(tempDir, "testdata_sample_640x360.mkv")
	if err := copyFile(srcFile, dstFile); err != nil {
		t.Fatalf("failed to copy test file: %v", err)
	}

	ctx := context.Background()
	setup(t, ctx, tempDir)
}

// dumpContainerLogs reads and logs all output from a container
func dumpContainerLogs(t *testing.T, ctx context.Context, container testcontainers.Container, name string) {
	logs, err := container.Logs(ctx)
	if err != nil {
		t.Logf("failed to get %s container logs: %v", name, err)
		return
	}
	defer logs.Close()

	logBytes, err := io.ReadAll(logs)
	if err != nil {
		t.Logf("failed to read %s container logs: %v", name, err)
		return
	}

	t.Logf("=== %s container logs ===\n%s", name, string(logBytes))
}

// setup starts the various container that are necessary for this test.
// It returns a host:port string for the workflow server.
func setup(t *testing.T, ctx context.Context, tempDir string) string {
	// Create Docker network
	net, err := network.New(ctx, network.WithCheckDuplicate())
	if err != nil {
		t.Fatalf("failed to create network: %v", err)
	}
	networkName := net.Name

	// Set up transcoder service and it's deps.
	transcoderDbName := "videotranscoder"
	const dbUser = "postgres"
	const dbPassword = "postgres"
	transcoderPostgresReq := testcontainers.ContainerRequest{
		Image:        "postgres:16",
		ExposedPorts: []string{"5432/tcp"},
		Env: map[string]string{
			"POSTGRES_DB":       transcoderDbName,
			"POSTGRES_USER":     dbUser,
			"POSTGRES_PASSWORD": dbPassword,
		},
		Networks:       []string{networkName},
		NetworkAliases: map[string][]string{networkName: {"transcoderPostgres"}},
		WaitingFor:     wait.ForLog("database system is ready to accept connections").WithOccurrence(2),
	}
	transcoderPostgresContainer, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: transcoderPostgresReq,
		Started:          true,
	})
	if err != nil {
		t.Fatalf("failed to start postgres container: %v", err)
	}
	t.Cleanup(func() {
		dumpContainerLogs(t, ctx, transcoderPostgresContainer, "transcoderPostgres")
	})
	transcoderServerReq := testcontainers.ContainerRequest{
		Image: "krelinga/video-transcoder:" + transcodeTag(t) + "-server",
		Env: map[string]string{
			"VT_DB_HOST":     "transcoderPostgres",
			"VT_DB_PORT":     "5432",
			"VT_DB_USER":     dbUser,
			"VT_DB_PASSWORD": dbPassword,
			"VT_DB_NAME":     transcoderDbName,
			"VT_SERVER_PORT": "8080",
		},
		Networks:       []string{networkName},
		NetworkAliases: map[string][]string{networkName: {"transcoderserver"}},
		WaitingFor:     wait.ForLog("Starting HTTP server on port 8080"),
	}
	transcoderServerContainer, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: transcoderServerReq,
		Started:          true,
	})
	if err != nil {
		t.Fatalf("failed to start transcoder server container: %v", err)
	}
	t.Cleanup(func() {
		dumpContainerLogs(t, ctx, transcoderServerContainer, "transcoderServer")
	})
	transcoderWorkerReq := testcontainers.ContainerRequest{
		Image: "krelinga/video-transcoder:" + transcodeTag(t) + "-worker",
		Env: map[string]string{
			"VT_DB_HOST":     "transcoderPostgres",
			"VT_DB_PORT":     "5432",
			"VT_DB_USER":     dbUser,
			"VT_DB_PASSWORD": dbPassword,
			"VT_DB_NAME":     transcoderDbName,
		},
		Networks:       []string{networkName},
		NetworkAliases: map[string][]string{networkName: {"transcoderworker"}},
		Mounts: testcontainers.Mounts(
			testcontainers.BindMount(tempDir, "/nas/media"),
		),
		WaitingFor:     wait.ForLog("Worker started"),
	}
	transcoderWorkerContainer, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: transcoderWorkerReq,
		Started:          true,
	})
	if err != nil {
		t.Fatalf("failed to start transcoder worker container: %v", err)
	}
	t.Cleanup(func() {
		dumpContainerLogs(t, ctx, transcoderWorkerContainer, "transcoderWorker")
	})

	// Set up video info service and it's deps.
	videoInfoDbName := "videoinfo"
	videoInfoPostgresReq := testcontainers.ContainerRequest{
		Image:        "postgres:16",
		ExposedPorts: []string{"5432/tcp"},
		Env: map[string]string{
			"POSTGRES_DB":       videoInfoDbName,
			"POSTGRES_USER":     dbUser,
			"POSTGRES_PASSWORD": dbPassword,
		},
		Networks:       []string{networkName},
		NetworkAliases: map[string][]string{networkName: {"videoinfopostgres"}},
		WaitingFor:     wait.ForLog("database system is ready to accept connections").WithOccurrence(2),
	}
	videoInfoPostgresContainer, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: videoInfoPostgresReq,
		Started:          true,
	})
	if err != nil {
		t.Fatalf("failed to start video info postgres container: %v", err)
	}
	t.Cleanup(func() {
		dumpContainerLogs(t, ctx, videoInfoPostgresContainer, "videoInfoPostgres")
	})
	videoInfoServerReq := testcontainers.ContainerRequest{
		Image: "krelinga/video-info:" + videoInfoTag(t) + "-server",
		Env: map[string]string{
			"VI_DB_HOST":     "videoinfopostgres",
			"VI_DB_PORT":     "5432",
			"VI_DB_USER":     dbUser,
			"VI_DB_PASSWORD": dbPassword,
			"VI_DB_NAME":     videoInfoDbName,
			"VI_SERVER_PORT": "8080",
		},
		Networks:       []string{networkName},
		NetworkAliases: map[string][]string{networkName: {"videoinfoserver"}},
		WaitingFor:     wait.ForLog("Starting HTTP server on port 8080"),
	}
	videoInfoServerContainer, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: videoInfoServerReq,
		Started:          true,
	})
	if err != nil {
		t.Fatalf("failed to start video info server container: %v", err)
	}
	t.Cleanup(func() {
		dumpContainerLogs(t, ctx, videoInfoServerContainer, "videoInfoServer")
	})
	videoInfoWorkerReq := testcontainers.ContainerRequest{
		Image: "krelinga/video-info:" + videoInfoTag(t) + "-worker",
		Env: map[string]string{
			"VI_DB_HOST":     "videoinfopostgres",
			"VI_DB_PORT":     "5432",
			"VI_DB_USER":     dbUser,
			"VI_DB_PASSWORD": dbPassword,
			"VI_DB_NAME":     videoInfoDbName,
		},
		Mounts: testcontainers.Mounts(
			testcontainers.BindMount(tempDir, "/nas/media"),
		),
		Networks:       []string{networkName},
		NetworkAliases: map[string][]string{networkName: {"videinfoworker"}},
		WaitingFor:     wait.ForLog("Worker started"),
	}
	videoInfoWorkerContainer, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: videoInfoWorkerReq,
		Started:          true,
	})
	if err != nil {
		t.Fatalf("failed to start video info worker container: %v", err)
	}
	t.Cleanup(func() {
		dumpContainerLogs(t, ctx, videoInfoWorkerContainer, "videoInfoWorker")
	})

	// Start workflows postgres
	workflowsDbName := "videoworkflows"
	workflowsPostgresReq := testcontainers.ContainerRequest{
		Image:        "postgres:16",
		ExposedPorts: []string{"5432/tcp"},
		Env: map[string]string{
			"POSTGRES_DB":       workflowsDbName,
			"POSTGRES_USER":     dbUser,
			"POSTGRES_PASSWORD": dbPassword,
		},
		Networks:       []string{networkName},
		NetworkAliases: map[string][]string{networkName: {"workflowspostgres"}},
		WaitingFor:     wait.ForLog("database system is ready to accept connections").WithOccurrence(2),
	}
	workflowsPostgresContainer, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: workflowsPostgresReq,
		Started:          true,
	})
	if err != nil {
		t.Fatalf("failed to start workflows postgres container: %v", err)
	}
	t.Cleanup(func() {
		dumpContainerLogs(t, ctx, workflowsPostgresContainer, "workflowsPostgres")
	})

	// Start Temporal server with auto-setup (bootstraps postgres schema)
	temporalReq := testcontainers.ContainerRequest{
		Image: "temporalio/auto-setup:latest",
		Env: map[string]string{
			"DB":             "postgres12",
			"DB_PORT":        "5432",
			"POSTGRES_USER":  dbUser,
			"POSTGRES_PWD":   dbPassword,
			"POSTGRES_SEEDS": "workflowspostgres",
		},
		ExposedPorts:   []string{"7233/tcp"},
		Networks:       []string{networkName},
		NetworkAliases: map[string][]string{networkName: {"temporal"}},
		WaitingFor:     wait.ForLog("Temporal server started.").WithStartupTimeout(2 * time.Minute),
	}
	temporalContainer, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: temporalReq,
		Started:          true,
	})
	if err != nil {
		t.Fatalf("failed to start temporal container: %v", err)
	}
	t.Cleanup(func() {
		dumpContainerLogs(t, ctx, temporalContainer, "temporal")
	})

	// Build and start worker container.
	workerReq := testcontainers.ContainerRequest{
		FromDockerfile: testcontainers.FromDockerfile{
			Context:    ".",
			Dockerfile: "Dockerfile",
			BuildArgs:  map[string]*string{},
			BuildOptionsModifier: func(buildOptions *build.ImageBuildOptions) {
				buildOptions.Target = "worker"
			},
		},
		ExposedPorts:   []string{"8080/tcp"},
		Env:            map[string]string{
			"VW_TEMPORAL_HOST":   "temporal",
			"VW_TEMPORAL_PORT":   "7233",
			"VW_TRANSCODE_HOST":  "transcoderserver",
			"VW_TRANSCODE_PORT":  "8080",
			"VW_VIDEOINFO_HOST":  "videoinfoserver",
			"VW_VIDEOINFO_PORT":  "8080",
		},
		Networks:       []string{networkName},
		NetworkAliases: map[string][]string{networkName: {"worker"}},
		WaitingFor:     wait.ForLog("Starting worker on task queue"),
	}
	workerContainer, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: workerReq,
		Started:          true,
	})
	if err != nil {
		t.Fatalf("failed to start worker container: %v", err)
	}
	t.Cleanup(func() {
		dumpContainerLogs(t, ctx, workerContainer, "worker")
	})

	// Build and start server container.
	serverReq := testcontainers.ContainerRequest{
		FromDockerfile: testcontainers.FromDockerfile{
			Context:    ".",
			Dockerfile: "Dockerfile",
			BuildArgs:  map[string]*string{},
			BuildOptionsModifier: func(buildOptions *build.ImageBuildOptions) {
				buildOptions.Target = "server"
			},
		},
		ExposedPorts:   []string{"8080/tcp"},
		Env:            map[string]string{
			"VW_TEMPORAL_HOST":   "temporal",
			"VW_TEMPORAL_PORT":   "7233",
			"VW_LIBRARY_PATH":    "/data/library", // TODO: mount a volume here and add test files
		},
		Networks:       []string{networkName},
		NetworkAliases: map[string][]string{networkName: {"server"}},
		WaitingFor:     wait.ForLog("Starting server on :8080"),
	}
	serverContainer, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: serverReq,
		Started:          true,
	})
	if err != nil {
		t.Fatalf("failed to start server container: %v", err)
	}
	t.Cleanup(func() {
		dumpContainerLogs(t, ctx, serverContainer, "server")
	})

	host, err := serverContainer.Host(ctx)
	if err != nil {
		t.Fatalf("failed to get server container host: %v", err)
	}
	port, err := serverContainer.MappedPort(ctx, "8080")
	if err != nil {
		t.Fatalf("failed to get server container port: %v", err)
	}

	return fmt.Sprintf("%s:%s", host, port.Port())
}

// copyFile copies a file from src to dst
func copyFile(src, dst string) error {
	srcFile, err := os.Open(src)
	if err != nil {
		return err
	}
	defer srcFile.Close()

	dstFile, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer dstFile.Close()

	_, err = io.Copy(dstFile, srcFile)
	return err
}
