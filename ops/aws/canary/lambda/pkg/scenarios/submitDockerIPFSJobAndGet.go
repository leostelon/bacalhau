package scenarios

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/rs/zerolog/log"

	cmdutil "github.com/bacalhau-project/bacalhau/cmd/util"
	"github.com/bacalhau-project/bacalhau/pkg/config/types"
	"github.com/bacalhau-project/bacalhau/pkg/downloader"
	"github.com/bacalhau-project/bacalhau/pkg/downloader/util"
	"github.com/bacalhau-project/bacalhau/pkg/publicapi/apimodels"
	clientv2 "github.com/bacalhau-project/bacalhau/pkg/publicapi/client/v2"
	"github.com/bacalhau-project/bacalhau/pkg/system"
)

// This test submits a job that uses the Docker executor with an IPFS input.
func SubmitDockerIPFSJobAndGet(ctx context.Context, cfg types.BacalhauConfig) error {
	apiV1, err := cmdutil.GetAPIClient(cfg)
	if err != nil {
		return err
	}
	apiv2 := clientv2.New(fmt.Sprintf("http://%s:%d", cfg.Node.ClientAPI.Host, cfg.Node.ClientAPI.Port))

	cm := system.NewCleanupManager()
	j, err := getSampleDockerIPFSJob()
	if err != nil {
		return err
	}

	expectedChecksum := "ea1efa312267e09809ae13f311970863  /inputs/data.tar.gz"
	expectedStat := "62731802"
	// Tests use the cid of the file we uploaded in scenarios_test.go
	if os.Getenv("BACALHAU_CANARY_TEST_CID") != "" {
		j.Spec.Inputs[0].CID = os.Getenv("BACALHAU_CANARY_TEST_CID")
		expectedChecksum = "c639efc1e98762233743a75e7798dd9c  /inputs/data.tar.gz"
		expectedStat = "21"
	}

	submittedJob, err := apiV1.Submit(ctx, j)
	if err != nil {
		return err
	}

	log.Ctx(ctx).Info().Msgf("submitted job: %s", submittedJob.Metadata.ID)

	err = waitUntilCompleted(ctx, apiV1, submittedJob)
	if err != nil {
		return fmt.Errorf("waiting until completed: %s", err)
	}

	results, err := apiv2.Jobs().Results(ctx, &apimodels.ListJobResultsRequest{
		JobID: submittedJob.Metadata.ID,
	})
	if err != nil {
		return fmt.Errorf("getting results: %s", err)
	}

	if len(results.Results) == 0 {
		return fmt.Errorf("no results found")
	}

	outputDir, err := os.MkdirTemp(os.TempDir(), "submitAndGet")
	if err != nil {
		return fmt.Errorf("making temporary dir: %s", err)
	}
	defer os.RemoveAll(outputDir)

	downloadSettings, err := getIPFSDownloadSettings()
	if err != nil {
		return fmt.Errorf("getting download settings: %s", err)
	}
	downloadSettings.OutputDir = outputDir
	// canary is running every 5 minutes with a 5 minutes timeout. It should be safe to allow the download to take up to 4 minutes and leave
	// 1 minute for the rest of the test
	downloadSettings.Timeout = 240 * time.Second

	downloaderProvider := util.NewStandardDownloaders(cm, cfg.Node.IPFS)
	if err != nil {
		return err
	}

	err = downloader.DownloadResults(ctx, results.Results, downloaderProvider, downloadSettings)
	if err != nil {
		return fmt.Errorf("downloading job: %s", err)
	}
	files, err := os.ReadDir(filepath.Join(downloadSettings.OutputDir, j.Spec.Outputs[0].Name))
	if err != nil {
		return fmt.Errorf("reading results directory: %s", err)
	}

	for _, file := range files {
		log.Ctx(ctx).Debug().Msgf("downloaded files: %s", file.Name())
	}
	if len(files) != 3 {
		return fmt.Errorf("expected 3 files in output dir, got %d", len(files))
	}
	body, err := os.ReadFile(filepath.Join(downloadSettings.OutputDir, j.Spec.Outputs[0].Name, "checksum.txt"))
	if err != nil {
		return err
	}

	// Tests use the checksum of the data we uploaded in scenarios_test.go
	err = compareOutput(body, expectedChecksum)
	if err != nil {
		return fmt.Errorf("testing md5 of input: %s", err)
	}
	body, err = os.ReadFile(filepath.Join(downloadSettings.OutputDir, j.Spec.Outputs[0].Name, "stat.txt"))
	if err != nil {
		return err
	}
	// Tests use the stat of the data we uploaded in scenarios_test.go
	err = compareOutput(body, expectedStat)
	if err != nil {
		return fmt.Errorf("testing ls of input: %s", err)
	}

	return nil
}
