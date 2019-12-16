package pod

import (
	"io"

	"sigs.k8s.io/kind/pkg/exec"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/client-go/kubernetes/scheme"
	coreV1Client "k8s.io/client-go/kubernetes/typed/core/v1"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/remotecommand"
)

// containerCmder implements exec.Cmder for kubernetes containers
type containerCmder struct {
	coreV1Client  *coreV1Client.CoreV1Client
	config        *rest.Config
	podName       string
	namespace     string
	containerName string
}

// ContainerCmder creates a new exec.Cmder against a kubernetes containers
func ContainerCmder(coreV1Client *coreV1Client.CoreV1Client, config *rest.Config, podName, namespace, containerName string) exec.Cmder {
	return &containerCmder{
		coreV1Client:  coreV1Client,
		config:        config,
		podName:       podName,
		namespace:     namespace,
		containerName: containerName,
	}
}

func (c *containerCmder) Command(command string, args ...string) exec.Cmd {
	return &containerCmd{
		coreV1Client:  c.coreV1Client,
		config:        c.config,
		podName:       c.podName,
		namespace:     c.namespace,
		containerName: c.containerName,
		command:       append([]string{command}, args...),
	}
}

// v implements exec.Cmd for kind pods
type containerCmd struct {
	coreV1Client  *coreV1Client.CoreV1Client
	config        *rest.Config
	podName       string
	namespace     string
	containerName string
	command       []string
	stdin         io.Reader
	stdout        io.Writer
	stderr        io.Writer
}

func (c *containerCmd) Run() error {

	stdin := false
	if c.stdin != nil {
		stdin = true
	}
	stdout := false
	if c.stdout != nil {
		stdout = true
	}
	stderr := false
	if c.stderr != nil {
		stderr = true
	}
	req := c.coreV1Client.RESTClient().Post().
		Resource("pods").
		Name(c.podName).
		Namespace(c.namespace).
		SubResource("exec").
		VersionedParams(&corev1.PodExecOptions{
			Container: c.containerName,
			Command:   c.command,
			Stdin:     stdin,
			Stdout:    stdout,
			Stderr:    stderr,
			TTY:       false,
		}, scheme.ParameterCodec)

	exec, err := remotecommand.NewSPDYExecutor(c.config, "POST", req.URL())
	if err != nil {
		return err
	}

	sopt := remotecommand.StreamOptions{
		Stdin:  c.stdin,
		Stdout: c.stdout,
		Stderr: c.stderr,
		Tty:    false,
	}
	err = exec.Stream(sopt)
	if err != nil {
		return err
	}

	return nil
}

func (c *containerCmd) SetEnv(env ...string) exec.Cmd {
	// kubectl exec does not support setting environment variables
	return c
}

func (c *containerCmd) SetStdin(r io.Reader) exec.Cmd {
	c.stdin = r
	return c
}

func (c *containerCmd) SetStdout(w io.Writer) exec.Cmd {
	c.stdout = w
	return c
}

func (c *containerCmd) SetStderr(w io.Writer) exec.Cmd {
	c.stderr = w
	return c
}
