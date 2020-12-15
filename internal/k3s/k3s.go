package k3s

import (
	"context"
	"fmt"
	"os/exec"
	"regexp"
	"strings"
	"time"

	"github.com/blang/semver/v4"
)

const (
	k3dMinVersion  string        = "1.19.0"
	defaultTimeout time.Duration = 10 * time.Second
)

//RunCmd executes a minikube command with given arguments
func RunCmd(verbose bool, timeout time.Duration, args ...string) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, "k3d", args...)

	outBytes, err := cmd.CombinedOutput()
	out := string(outBytes)
	if err != nil {
		return out, err
	}

	if ctx.Err() == context.DeadlineExceeded {
		return out, fmt.Errorf("Executing 'k3d %s' command with output '%s' timed out, try running the command manually or increasing timeout using the 'timeout' flag", strings.Join(args, " "), out)
	}

	if err != nil {
		if verbose {
			fmt.Printf("\nExecuted command:\n  k3d %s\nwith output:\n  %s\nand error:\n  %s\n", strings.Join(args, " "), string(out), err)
		}
		return out, fmt.Errorf("Executing the 'k3d %s' command with output '%s' and error message '%s' failed", strings.Join(args, " "), out, err)
	}
	if verbose {
		fmt.Printf("\nExecuted command:\n  k3d %s\nwith output:\n  %s\n", strings.Join(args, " "), string(out))
	}
	return out, nil
}

//CheckVersion checks whether minikube version is supported
func CheckVersion(verbose bool) error {
	versionOutput, err := RunCmd(verbose, defaultTimeout, "version")
	if err != nil {
		return err
	}

	exp, _ := regexp.Compile("k3s version v([^\\s-]+)")
	versionString := exp.FindStringSubmatch(versionOutput)
	if verbose {
		fmt.Printf("Extracted K3s version: '%s'", versionString[1])
	}
	version, err := semver.Parse(versionString[1])
	if err != nil {
		return err
	}

	minVersion, _ := semver.Parse(k3dMinVersion)
	if version.LT(minVersion) {
		return fmt.Errorf("You are using an unsupported k3s version '%s'. "+
			"This may not work. The recommended k3s version is '%s'", version, minVersion)
	}

	return nil
}

//Initialize verifies whether the k3d CLI tool is properly installed
func Initialize(verbose bool) error {
	//ensure k3d is in PATH
	if _, err := exec.LookPath("k3d"); err != nil {
		if verbose {
			fmt.Printf("Command 'k3d' not found in PATH")
		}
		return err
	}

	//verify whether k3d seems to be properly installed
	if _, err := RunCmd(verbose, defaultTimeout, "cluster", "list"); err != nil {
		return err
	}

	return nil
}

//ClusterExists checks whether a cluster exists
func ClusterExists(verbose bool, clusterName string) (bool, error) {
	args := []string{"cluster", "list", "-o", "json"}
	clusterJSON, err := RunCmd(verbose, defaultTimeout, args...)
	if err != nil {
		return false, err
	}
	if verbose {
		fmt.Printf("K3d cluster list JSON: '%s'", clusterJSON)
	}

	clusterList := &clusterList{}
	if err := clusterList.Unmarshal([]byte(clusterJSON)); err != nil {
		return false, err
	}

	for _, cluster := range clusterList.clusters {
		if cluster.name == clusterName {
			return true, nil
		}
	}
	return false, nil
}

//StartCluster starts a cluster
func StartCluster(verbose bool, timeout time.Duration, clusterName string) error {
	output, err := RunCmd(verbose, timeout,
		"cluster", "create", clusterName,
		"--timeout", fmt.Sprintf("%d", timeout.Round(time.Second)),
		"-p", "80:80@loadbalancer", "-p", "443:80@loadbalancer",
	)
	if verbose {
		fmt.Printf("K3d cluster creation output: '%s'", output)
	}
	if err != nil {
		return err
	}
	return nil
}

//DeleteCluster deletes a cluster
func DeleteCluster(verbose bool, timeout time.Duration, clusterName string) error {
	output, err := RunCmd(verbose, timeout, "cluster", "delete", clusterName)
	if verbose {
		fmt.Printf("K3d cluster start output: '%s'", output)
	}
	if err != nil {
		return err
	}
	return nil
}
