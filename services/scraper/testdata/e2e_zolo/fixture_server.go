package main

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"runtime"
	"syscall"
	"time"
)

func main() {
	page1, page2, err := loadFixtures()
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to load fixtures: %v\n", err)
		os.Exit(1)
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet && r.Method != http.MethodHead {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}

		switch r.URL.Path {
		case "/":
			serveText(w, r, "ok")
		case "/calgary-real-estate", "/calgary-real-estate/", "/calgary-real-estate/page-1", "/calgary-real-estate/page-1/":
			serveHTML(w, r, page1)
		case "/calgary-real-estate/page-2", "/calgary-real-estate/page-2/":
			serveHTML(w, r, page2)
		default:
			http.NotFound(w, r)
		}
	})

	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		fmt.Fprintf(os.Stderr, "listen failed: %v\n", err)
		os.Exit(1)
	}
	baseURL := fmt.Sprintf("http://%s", listener.Addr().String())
	fmt.Printf("FIXTURE_BASE_URL=%s\n", baseURL)

	server := &http.Server{Handler: mux}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	go func() {
		<-ctx.Done()
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = server.Shutdown(shutdownCtx)
	}()

	if err := server.Serve(listener); err != nil && !errors.Is(err, http.ErrServerClosed) {
		fmt.Fprintf(os.Stderr, "server error: %v\n", err)
		os.Exit(1)
	}
}

func loadFixtures() ([]byte, []byte, error) {
	baseDir, err := fixtureDir()
	if err != nil {
		return nil, nil, err
	}
	page1, err := os.ReadFile(filepath.Join(baseDir, "page1.html"))
	if err != nil {
		return nil, nil, err
	}
	page2, err := os.ReadFile(filepath.Join(baseDir, "page2.html"))
	if err != nil {
		return nil, nil, err
	}
	return page1, page2, nil
}

func fixtureDir() (string, error) {
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		return "", fmt.Errorf("unable to resolve fixture directory")
	}
	return filepath.Dir(file), nil
}

func serveHTML(w http.ResponseWriter, r *http.Request, body []byte) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	if r.Method == http.MethodHead {
		return
	}
	_, _ = w.Write(body)
}

func serveText(w http.ResponseWriter, r *http.Request, body string) {
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	if r.Method == http.MethodHead {
		return
	}
	_, _ = w.Write([]byte(body))
}
