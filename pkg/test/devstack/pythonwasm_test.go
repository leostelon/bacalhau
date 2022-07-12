package devstack

import (
	"fmt"
	"io/ioutil"
	"os"
	"strings"
	"testing"
	"time"

	cmd "github.com/filecoin-project/bacalhau/cmd/bacalhau"
	"github.com/filecoin-project/bacalhau/pkg/computenode"
	"github.com/filecoin-project/bacalhau/pkg/devstack"
	"github.com/filecoin-project/bacalhau/pkg/executor"
	_ "github.com/filecoin-project/bacalhau/pkg/logger"
	"github.com/rs/zerolog/log"
	"github.com/stretchr/testify/require"
)

// full end-to-end test of python/wasm:
//
// * use CLI to submit a python job with --deterministic set
// * context (python file and requirements.txt) should be pinned to ipfs by
//   requester node
// * docker executor downloads context and starts wasm container image with the
//   context mounted in

func TestSimplestPythonWasmDashC(t *testing.T) {
	ctx, span := newSpan("TestSimplestPythonWasmDashC")
	defer span.End()
	stack, cm := SetupTest(t, 1, 0, computenode.NewDefaultComputeNodeConfig())
	defer TeardownTest(stack, cm)

	nodeIds, err := stack.GetNodeIds()
	require.NoError(t, err)

	// TODO: see also list_test.go, maybe factor out a common way to do this cli
	// setup
	_, out, err := cmd.ExecuteTestCobraCommand(t, cmd.RootCmd,
		fmt.Sprintf("--api-port=%d", stack.Nodes[0].APIServer.Port),
		"--api-host=localhost",
		"run",
		"python",
		"--deterministic",
		"-c",
		"print(1+1)",
	)
	require.NoError(t, err)

	jobId := strings.TrimSpace(out)
	log.Debug().Msgf("jobId=%s", jobId)
	// wait for the job to complete across all nodes
	err = stack.WaitForJob(ctx, jobId,
		devstack.WaitForJobThrowErrors([]executor.JobStateType{
			executor.JobStateBidRejected,
			executor.JobStateError,
		}),
		devstack.WaitForJobAllHaveState(nodeIds, executor.JobStateComplete),
	)
	require.NoError(t, err)

	// load result from ipfs and check it
	// TODO: see devStackDockerStorageTest for how to do this

}

// TODO: test that > 10MB context is rejected

func TestSimplePythonWasm(t *testing.T) {
	ctx, span := newSpan("TestSimplePythonWasm")
	defer span.End()
	stack, cm := SetupTest(t, 1, 0, computenode.NewDefaultComputeNodeConfig())
	defer TeardownTest(stack, cm)

	nodeIds, err := stack.GetNodeIds()
	require.NoError(t, err)
	tmpDir, err := ioutil.TempDir("", "devstack_test")
	require.NoError(t, err)
	defer func() {
		err := os.RemoveAll(tmpDir)
		require.NoError(t, err)
	}()

	oldDir, err := os.Getwd()
	require.NoError(t, err)
	err = os.Chdir(tmpDir)
	require.NoError(t, err)
	defer func() {
		err := os.Chdir(oldDir)
		require.NoError(t, err)
	}()

	// write bytes to main.py
	mainPy := []byte("print(1+1)")
	err = ioutil.WriteFile("main.py", mainPy, 0644)
	require.NoError(t, err)

	_, out, err := cmd.ExecuteTestCobraCommand(t, cmd.RootCmd,
		fmt.Sprintf("--api-port=%d", stack.Nodes[0].APIServer.Port),
		"--api-host=localhost",
		"run",
		"python",
		"--deterministic",
		"main.py",
	)
	require.NoError(t, err)
	jobId := strings.TrimSpace(out)
	log.Debug().Msgf("jobId=%s", jobId)
	time.Sleep(time.Second * 5)
	err = stack.WaitForJob(ctx, jobId,
		devstack.WaitForJobThrowErrors([]executor.JobStateType{
			executor.JobStateBidRejected,
			executor.JobStateError,
		}),
		devstack.WaitForJobAllHaveState(nodeIds, executor.JobStateComplete),
	)
	require.NoError(t, err)

}

// func TestPythonWasmWithRequirements(t *testing.T) {
// 	tmpDir, err := ioutil.TempDir("", "devstack_test")
// 	require.NoError(t, err)
// 	defer func() {
// 		err := os.RemoveAll(tmpDir)
// 		require.NoError(t, err)
// 	}()

// 	oldDir, err := os.Getwd()
// 	require.NoError(t, err)
// 	err = os.Chdir(tmpDir)
// 	require.NoError(t, err)
// 	defer func() {
// 		err := os.Chdir(oldDir)
// 		require.NoError(t, err)
// 	}()

// 	_, out, err := ExecuteTestCobraCommand(t, RootCmd,
// 		"run",
// 		"python",
// 		"--deterministic",
// 		"main.py",
// 	)
// 	require.NoError(t, err)

// }