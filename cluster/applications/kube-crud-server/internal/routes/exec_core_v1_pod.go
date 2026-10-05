package routes

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"sync"

	"github.com/gorilla/websocket"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/remotecommand"
	"k8s.io/client-go/util/exec"
)

var execUpgrader = websocket.Upgrader{
	ReadBufferSize:  4096,
	WriteBufferSize: 4096,
	CheckOrigin:     func(r *http.Request) bool { return true },
}

type wsInMessage struct {
	Type string `json:"type"`
	Data string `json:"data,omitempty"`
	Cols uint16 `json:"cols,omitempty"`
	Rows uint16 `json:"rows,omitempty"`
}

type wsStreamMessage struct {
	Type string `json:"type"`
	Data string `json:"data"`
}

type wsExitMessage struct {
	Type string `json:"type"`
	Code int    `json:"code"`
}

type wsStreamWriter struct {
	conn       *websocket.Conn
	channel    string
	writeMutex *sync.Mutex
}

func (w *wsStreamWriter) Write(p []byte) (int, error) {
	w.writeMutex.Lock()
	defer w.writeMutex.Unlock()

	if err := w.conn.WriteJSON(wsStreamMessage{Type: w.channel, Data: string(p)}); err != nil {
		return 0, err
	}
	return len(p), nil
}

type terminalSizeQueue struct {
	ch chan remotecommand.TerminalSize
}

func (q *terminalSizeQueue) Next() *remotecommand.TerminalSize {
	size, ok := <-q.ch
	if !ok {
		return nil
	}
	return &size
}

func ExecCoreV1Pod(clientset *kubernetes.Clientset, kubeConfig *rest.Config) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		namespace := r.PathValue("namespace")
		name := r.PathValue("name")
		container := r.URL.Query().Get("container")

		command := r.URL.Query()["command"]
		if len(command) == 0 {
			command = []string{"sh"}
		}

		if !websocket.IsWebSocketUpgrade(r) {
			request := clientset.CoreV1().RESTClient().Post().
				Resource("pods").
				Name(name).
				Namespace(namespace).
				SubResource("exec").
				VersionedParams(&corev1.PodExecOptions{
					Container: container,
					Command:   command,
					Stdout:    true,
					Stderr:    true,
				}, scheme.ParameterCodec)

			executor, err := remotecommand.NewSPDYExecutor(kubeConfig, http.MethodPost, request.URL())
			if err != nil {
				slog.Error("failed to create executor", "error", err)
				http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
				return
			}

			var stderr bytes.Buffer
			if err := executor.StreamWithContext(r.Context(), remotecommand.StreamOptions{
				Stdout: w,
				Stderr: &stderr,
			}); err != nil {
				if errors.Is(err, context.Canceled) {
					return
				}
				slog.Error("failed to exec", "error", err, "stderr", stderr.String())
			}
			return
		}

		conn, err := execUpgrader.Upgrade(w, r, nil)
		if err != nil {
			slog.Error("failed to upgrade websocket", "error", err)
			return
		}
		defer conn.Close()

		stdinReader, stdinWriter := io.Pipe()
		defer stdinReader.Close()

		q := &terminalSizeQueue{ch: make(chan remotecommand.TerminalSize, 1)}

		writeMutex := &sync.Mutex{}
		stdoutWriter := &wsStreamWriter{conn: conn, channel: "stdout", writeMutex: writeMutex}
		stderrWriter := &wsStreamWriter{conn: conn, channel: "stderr", writeMutex: writeMutex}

		ctx, cancel := context.WithCancel(r.Context())
		defer cancel()

		go func() {
			defer close(q.ch)
			defer cancel()
			defer stdinWriter.Close()
			for {
				_, payload, err := conn.ReadMessage()
				if err != nil {
					return
				}
				var in wsInMessage
				if err := json.Unmarshal(payload, &in); err != nil {
					continue
				}
				switch in.Type {
				case "stdin":
					if _, err := stdinWriter.Write([]byte(in.Data)); err != nil {
						return
					}
				case "resize":
					select {
					case q.ch <- remotecommand.TerminalSize{Width: in.Cols, Height: in.Rows}:
					default:
					}
				}
			}
		}()

		request := clientset.CoreV1().RESTClient().Post().
			Resource("pods").
			Name(name).
			Namespace(namespace).
			SubResource("exec").
			VersionedParams(&corev1.PodExecOptions{
				Container: container,
				Command:   command,
				Stdin:     true,
				Stdout:    true,
				Stderr:    true,
				TTY:       true,
			}, scheme.ParameterCodec)

		executor, err := remotecommand.NewSPDYExecutor(kubeConfig, http.MethodPost, request.URL())
		if err != nil {
			slog.Error("failed to create executor", "error", err)
			return
		}

		exitCode := 0
		if err := executor.StreamWithContext(ctx, remotecommand.StreamOptions{
			Stdin:             stdinReader,
			Stdout:            stdoutWriter,
			Stderr:            stderrWriter,
			Tty:               true,
			TerminalSizeQueue: q,
		}); err != nil {
			var codeErr exec.CodeExitError
			if errors.As(err, &codeErr) {
				exitCode = codeErr.Code
			} else if errors.Is(err, context.Canceled) {
				return
			} else {
				slog.Error("failed to exec", "error", err)
				exitCode = -1
			}
		}

		writeMutex.Lock()
		_ = conn.WriteJSON(wsExitMessage{Type: "exit", Code: exitCode})
		writeMutex.Unlock()
	}
}
