package k8s

import (
	"bytes"
	"io"
	"net/url"

	errors2 "github.com/pkg/errors"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/remotecommand"
	"sigs.k8s.io/controller-runtime/pkg/client/config"

	"github.com/fedstate/fedstate/pkg/logi"
	"github.com/fedstate/fedstate/pkg/util"
)

var k8sExecLog = logi.Log.Sugar().Named("kubernetesExec")

func GetConfig() *rest.Config {
	restConfig, err := config.GetConfig()
	if err != nil {
		panic(err)
	}
	return restConfig
}

func ExecCmd(config *rest.Config, pod *corev1.Pod, containerName string, command string) (string, string, error) {
	clientSet, err := kubernetes.NewForConfig(config)
	if err != nil {
		return "", "", err
	}
	req := clientSet.CoreV1().RESTClient().
		Post().
		Timeout(util.CtxTimeout).
		Resource("pods").
		Name(pod.Name).
		Namespace(pod.Namespace).
		SubResource("exec").
		VersionedParams(&corev1.PodExecOptions{
			Container: containerName,
			Command:   []string{"/bin/sh", "-c", command},
			Stdin:     false,
			Stdout:    true,
			Stderr:    true,
		}, scheme.ParameterCodec)

	k8sExecLog.Infof("execute command %s", command)

	stdout := new(bytes.Buffer)
	stderr := new(bytes.Buffer)

	err = util.TimeoutWrap(util.CtxTimeout, func() error {
		return execute("POST", req.URL(), config, nil, stdout, stderr,
			false)
	})

	outStr := stdout.String()
	errStr := stderr.String()
	k8sExecLog.Infof("stdout: %s, stderr: %s", outStr, errStr)

	if err != nil {
		// stdout要返回给上层
		return outStr, errStr, errors2.Wrap(err, "k8s exec err")
	}

	return outStr, errStr, nil
}

func execute(method string, url *url.URL, config *rest.Config, stdin io.Reader, stdout, stderr io.Writer, tty bool) error {
	k8sExecLog.Infof("req url: %v", url)
	exec, err := remotecommand.NewSPDYExecutor(config, method, url)
	if err != nil {
		return err
	}

	return exec.Stream(remotecommand.StreamOptions{
		Stdin:  stdin,
		Stdout: stdout,
		Stderr: stderr,
		Tty:    tty,
	})
}
