package phpfpm

import (
	"bufio"
	"errors"
	"io"
	"io/ioutil"
	"log"
	"net"
	"net/http"
	"net/textproto"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"syscall"
	"text/template"
	"time"
)

// DefaultConfig ...
var DefaultConfig = &Config{
	MaxChildren:     5,
	MinSpareServers: 1,
	MaxSpareServers: 3,
	StartServers:    3,
}

// Proc ...
type Proc struct {
	Addr    string
	Cfg     *Config
	Dir     string
	Process *os.Process
}

// Config ...
type Config struct {
	MaxChildren     int
	MinSpareServers int
	MaxSpareServers int
	StartServers    int
}

type configFile struct {
	*Config
	Addr     string
	ErrorLog string
}

type response struct {
	Header http.Header
	Body   []byte
}

var versionPat = regexp.MustCompile("^PHP 7\\.")

func readResponse(r io.Reader) (*response, error) {
	br := bufio.NewReader(r)
	tr := textproto.NewReader(br)
	mh, err := tr.ReadMIMEHeader()
	if err != nil {
		return nil, err
	}
	h := http.Header(mh)
	b, err := ioutil.ReadAll(br)
	if err != nil {
		return nil, err
	}

	return &response{
		Header: h,
		Body:   b,
	}, nil
}

func templateFromLines(src []string) (*template.Template, error) {
	return template.New("conf").Parse(strings.Join(src, "\n"))
}

func writeConfig(dst string, c *configFile) error {
	t, err := templateFromLines([]string{
		"[global]",
		"daemonize = no",
		"error_log = {{.ErrorLog}}",
		"[www]",
		"user = nobody",
		"listen = {{.Addr}}",
		"pm = dynamic",
		"pm.max_children = {{.MaxChildren}}",
		"pm.min_spare_servers = {{.MinSpareServers}}",
		"pm.max_spare_servers = {{.MaxSpareServers}}",
		"pm.start_servers = {{.StartServers}}",
	})
	if err != nil {
		return err
	}

	w, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer w.Close()

	return t.Execute(w, c)
}

func localAddr() (string, error) {
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return "", err
	}

	if err := l.Close(); err != nil {
		return "", err
	}

	return l.Addr().String(), nil
}

// Shutdown ...
func (p *Proc) Shutdown() error {
	// Kill the entire process group
	errA := syscall.Kill(-p.Process.Pid, syscall.SIGKILL)
	errB := os.RemoveAll(p.Dir)
	if errA != nil {
		return errA
	}
	return errB
}

// MustStart ...
func MustStart(cfg *Config) *Proc {
	p, err := Start(cfg)
	if err != nil {
		panic(err)
	}
	return p
}

func waitFor(addr string) error {
	for {
		c, err := net.Dial("tcp", addr)
		if err == nil {
			c.Close()
			return nil
		}

		time.Sleep(1 * time.Second)
	}
}

func canUseFPM(path string) bool {
	log.Println(path)
	cmd := exec.Command(path, "--version")

	r, err := cmd.StdoutPipe()
	if err != nil {
		return false
	}

	if err := cmd.Start(); err != nil {
		return false
	}

	b, err := ioutil.ReadAll(r)
	if err != nil {
		return false
	}

	if err := cmd.Wait(); err != nil {
		return false
	}

	return versionPat.Match(b)
}

func pathsToTry() []string {
	switch runtime.GOOS {
	case "darwin":
		return []string{
			"/usr/local/sbin/php-fpm",
			"/usr/local/sbin/php71-fpm",
			"/usr/local/sbin/php70-fpm",
			"php-fpm",
			"php71-fpm",
			"php70-fpm",
			"php-fpm7.0",
			"php-fpm7.1",
		}
	}
	return []string{
		"php-fpm",
		"php-fpm7.0",
		"php-fpm7.1",
	}
}

func findFPMPath() (string, error) {
	for _, path := range pathsToTry() {
		log.Println(path)
		p, err := exec.LookPath(path)
		if err != nil {
			continue
		}
		if !canUseFPM(p) {
			continue
		}
		return p, nil
	}
	return "", errors.New("could not find php-fpm")
}

// Start ...
func Start(cfg *Config) (*Proc, error) {
	tmp, err := ioutil.TempDir("", "")
	if err != nil {
		return nil, err
	}

	addr, err := localAddr()
	if err != nil {
		return nil, err
	}

	cf := filepath.Join(tmp, "conf")
	if err := writeConfig(cf, &configFile{
		Config:   cfg,
		Addr:     addr,
		ErrorLog: filepath.Join(tmp, "err"),
	}); err != nil {
		return nil, err
	}

	log.Println(cf)

	cmd, err := findFPMPath()
	if err != nil {
		return nil, err
	}

	log.Println(cmd)

	c := exec.Command(cmd, "-n", "-y", cf)
	c.SysProcAttr = &syscall.SysProcAttr{
		Setpgid: true,
	}
	c.Stderr = os.Stderr
	c.Stdout = os.Stdout

	if err := c.Start(); err != nil {
		return nil, err
	}

	if err := waitFor(addr); err != nil {
		return nil, err
	}

	return &Proc{
		Addr:    addr,
		Cfg:     cfg,
		Dir:     tmp,
		Process: c.Process,
	}, nil
}
