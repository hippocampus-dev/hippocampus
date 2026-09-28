package main

import (
	"archive/tar"
	"compress/gzip"
	"crypto/x509"
	"encoding/json"
	"encoding/pem"
	"flag"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"os/exec"
	"os/signal"
	"path"
	"regexp"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/golang-jwt/jwt/v5"
	expect "github.com/google/goexpect"
	"golang.org/x/xerrors"
)

func envOrDefaultValue[T any](key string, defaultValue T) T {
	value, exists := os.LookupEnv(key)
	if !exists {
		return defaultValue
	}

	switch any(defaultValue).(type) {
	case string:
		return any(value).(T)
	case int:
		if intValue, err := strconv.Atoi(value); err == nil {
			return any(intValue).(T)
		}
	case int64:
		if intValue, err := strconv.ParseInt(value, 10, 64); err == nil {
			return any(intValue).(T)
		}
	case uint:
		if uintValue, err := strconv.ParseUint(value, 10, 0); err == nil {
			return any(uint(uintValue)).(T)
		}
	case uint64:
		if uintValue, err := strconv.ParseUint(value, 10, 64); err == nil {
			return any(uintValue).(T)
		}
	case float64:
		if floatValue, err := strconv.ParseFloat(value, 64); err == nil {
			return any(floatValue).(T)
		}
	case bool:
		if boolValue, err := strconv.ParseBool(value); err == nil {
			return any(boolValue).(T)
		}
	case time.Duration:
		if durationValue, err := time.ParseDuration(value); err == nil {
			return any(durationValue).(T)
		}
	}

	return defaultValue
}

// Reserves the rest of the Pod's terminationGracePeriodSeconds in internal/controllers/runner_controller.go for getRemoveToken and remove.
const runnerExitTimeout = 60 * time.Second

type TokenResponse struct {
	Token     string `json:"token"`
	ExpiresAt string `json:"expires_at"`
}

func check() {
	if _, err := exec.LookPath("bash"); err != nil {
		log.Fatal(err)
	}
}

func install(runnerVersion string) {
	request, err := http.NewRequest("GET", fmt.Sprintf("https://github.com/actions/runner/releases/download/v%s/actions-runner-linux-x64-%s.tar.gz", runnerVersion, runnerVersion), nil)
	if err != nil {
		log.Fatal(err)
	}
	response, err := http.DefaultClient.Do(request)
	if err != nil {
		log.Fatal(err)
	}

	defer func() {
		_ = response.Body.Close()
	}()
	gzipReader, err := gzip.NewReader(response.Body)
	if err != nil {
		log.Fatal(err)
	}

	defer gzipReader.Close()
	tarReader := tar.NewReader(gzipReader)
	for {
		hdr, err := tarReader.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			log.Fatal(err)
		}
		if hdr.Typeflag != tar.TypeDir {
			if err := os.MkdirAll(path.Dir(hdr.Name), 0777); err != nil {
				log.Fatal(err)
			}
			f, err := os.Create(hdr.Name)
			if err != nil {
				log.Fatal(err)
			}
			func(f *os.File) {
				defer f.Close()

				if _, err := io.Copy(f, tarReader); err != nil {
					log.Fatal(err)
				}
			}(f)
			if err := os.Chmod(hdr.Name, os.FileMode(hdr.Mode)); err != nil {
				log.Fatal(err)
			}
		}
	}

	command := exec.Command("bash", "bin/installdependencies.sh")
	command.Stdout = os.Stdout
	command.Stderr = os.Stderr
	if err := command.Run(); err != nil {
		log.Fatal(err)
	}
}

func getRegistrationToken(target string, token string, isOrganization bool) string {
	if _, err := os.Stat(token); err == nil {
		tokenBytes, err := os.ReadFile(token)
		if err != nil {
			log.Fatalf("failed to read token: %v", err)
		}
		token = string(tokenBytes)
	}

	var url string
	if isOrganization {
		url = fmt.Sprintf("https://api.github.com/orgs/%s/actions/runners/registration-token", target)
	} else {
		url = fmt.Sprintf("https://api.github.com/repos/%s/actions/runners/registration-token", target)
	}

	request, err := http.NewRequest(http.MethodPost, url, nil)
	if err != nil {
		log.Fatal(err)
	}
	request.Header.Set("Authorization", fmt.Sprintf("Bearer %s", token))
	response, err := http.DefaultClient.Do(request)
	if err != nil {
		log.Fatal(err)
	}
	defer func() {
		_ = response.Body.Close()
	}()

	if response.StatusCode != http.StatusCreated {
		log.Fatalf("failed to get registration token: %d", response.StatusCode)
	}

	body, err := io.ReadAll(response.Body)
	if err != nil {
		log.Fatal(err)
	}

	var registrationTokenResponse TokenResponse
	if err := json.Unmarshal(body, &registrationTokenResponse); err != nil {
		log.Fatal(err)
	}

	return registrationTokenResponse.Token
}

func getRemoveToken(target string, token string, isOrganization bool) string {
	if _, err := os.Stat(token); err == nil {
		tokenBytes, err := os.ReadFile(token)
		if err != nil {
			log.Fatalf("failed to read token: %v", err)
		}
		token = string(tokenBytes)
	}

	var url string
	if isOrganization {
		url = fmt.Sprintf("https://api.github.com/orgs/%s/actions/runners/remove-token", target)
	} else {
		url = fmt.Sprintf("https://api.github.com/repos/%s/actions/runners/remove-token", target)
	}

	request, err := http.NewRequest(http.MethodPost, url, nil)
	if err != nil {
		log.Fatal(err)
	}
	request.Header.Set("Authorization", fmt.Sprintf("Bearer %s", token))
	response, err := http.DefaultClient.Do(request)
	if err != nil {
		log.Fatal(err)
	}
	defer func() {
		_ = response.Body.Close()
	}()

	if response.StatusCode != http.StatusCreated {
		log.Fatalf("failed to get remove token: %d", response.StatusCode)
	}

	body, err := io.ReadAll(response.Body)
	if err != nil {
		log.Fatal(err)
	}

	var removeTokenResponse TokenResponse
	if err := json.Unmarshal(body, &removeTokenResponse); err != nil {
		log.Fatal(err)
	}

	return removeTokenResponse.Token
}

func run(registrationToken string, repository string, hostname string, labels string, runnerGroup string, disableupdate bool) *exec.Cmd {
	var args []string
	if runnerGroup != "" {
		args = append(args, "--runnergroup", runnerGroup)
	}
	if disableupdate {
		args = append(args, "--disableupdate")
	}
	e, _, err := expect.Spawn(fmt.Sprintf("bash config.sh --labels %s --token %s --url https://github.com/%s %s", labels, registrationToken, repository, strings.Join(args, " ")), -1, expect.Verbose(true), expect.Tee(os.Stdout))
	if err != nil {
		log.Fatal(err)
	}
	// config.sh prompts for the runner group only where no --runnergroup answered it already, since GetArgOrPrompt returns a non-empty argument without printing its prompt: https://github.com/actions/runner/blob/v2.335.1/src/Runner.Listener/CommandSettings.cs
	// A negative timeout on Expect falls back to the spawn timeout, which the -1 above left at goexpect's DefaultTimeout of a minute: https://github.com/google/goexpect/blob/5b6988669ffa/expect.go
	// An Expect left ungated on a prompt an argument suppresses therefore reaches the log.Fatal below a minute into every start, with the Runner CR and its Deployment both reporting success
	if runnerGroup == "" {
		_, _, err = e.Expect(regexp.MustCompile("Enter the name of the runner group to add this runner to:"), -1)
		if err != nil {
			log.Fatal(err)
		}
		if err := e.Send("\n"); err != nil {
			log.Fatal(err)
		}
	}
	_, _, err = e.Expect(regexp.MustCompile("Enter the name of runner:"), -1)
	if err != nil {
		log.Fatal(err)
	}
	if err := e.Send(hostname + "\n"); err != nil {
		log.Fatal(err)
	}
	replaceOrWorkFolder := regexp.MustCompile(`Would you like to replace the existing runner\? \(Y/N\)|Enter name of work folder:`)
	out, _, err := e.Expect(replaceOrWorkFolder, -1)
	if err != nil {
		log.Fatal(err)
	}
	if replaceOrWorkFolder.FindString(out) == `Would you like to replace the existing runner? (Y/N)` {
		if err := e.Send("Y\n"); err != nil {
			log.Fatal(err)
		}
		_, _, err = e.Expect(regexp.MustCompile("Enter name of work folder:"), -1)
		if err != nil {
			log.Fatal(err)
		}
	}
	if err := e.Send("\n"); err != nil {
		log.Fatal(err)
	}
	_, _, err = e.Expect(regexp.MustCompile("Settings Saved."), -1)
	if err != nil {
		log.Fatal(err)
	}
	if err := e.Send("exit\n"); err != nil {
		log.Fatal(err)
	}
	command := exec.Command("bash", "run.sh")
	// run.sh relays signals to Runner.Listener only when this is set: https://github.com/actions/runner/blob/v2.335.1/src/Misc/layoutroot/run.sh
	command.Env = append(os.Environ(), "RUNNER_MANUALLY_TRAP_SIG=1")
	command.Stdout = os.Stdout
	command.Stderr = os.Stderr
	if err := command.Start(); err != nil {
		log.Printf("%+v", err)
		return nil
	}
	return command
}

func remove(registrationToken string) {
	command := exec.Command("bash", "config.sh", "remove", "--token", registrationToken)
	if err := command.Run(); err != nil {
		log.Printf("%+v", err)
	}
}

func main() {
	var runnerVersion string
	var repository string
	var organization string
	var hostname string
	var labels string
	var runnerGroup string
	var token string
	var githubAppId string
	var githubAppInstallationId string
	var githubAppPrivateKey string
	var onlyInstall bool
	var withoutInstall bool
	var disableupdate bool
	flag.StringVar(&runnerVersion, "runner-version", envOrDefaultValue("RUNNER_VERSION", "2.335.1"), "Version of GitHub Actions runner")
	flag.StringVar(&repository, "repository", envOrDefaultValue("REPOSITORY", ""), "GitHub Repository Name (owner/repo)")
	flag.StringVar(&organization, "organization", envOrDefaultValue("ORGANIZATION", ""), "GitHub Organization Name")
	flag.StringVar(&hostname, "hostname", envOrDefaultValue("HOSTNAME", "runner"), "Hostname used as Runner name")
	flag.StringVar(&labels, "labels", envOrDefaultValue("LABELS", "github-actions-runner-controller"), "Comma-separated labels registered as runs-on targets")
	flag.StringVar(&runnerGroup, "runner-group", envOrDefaultValue("RUNNER_GROUP", ""), "GitHub runner group the runner joins, taking the default group when empty")
	flag.StringVar(&token, "token", envOrDefaultValue("TOKEN", "********"), "GitHub Token")
	flag.StringVar(&githubAppId, "github-app-id", envOrDefaultValue("GITHUB_APP_ID", ""), "GitHub App ID")
	flag.StringVar(&githubAppInstallationId, "github-app-installation-id", envOrDefaultValue("GITHUB_APP_INSTALLATION_ID", ""), "GitHub App Installation ID")
	flag.StringVar(&githubAppPrivateKey, "github-app-private-key", envOrDefaultValue("GITHUB_APP_PRIVATE_KEY", ""), "GitHub App Private Key")
	flag.BoolVar(&onlyInstall, "only-install", envOrDefaultValue("ONLY_INSTALL", false), "Execute install only")
	flag.BoolVar(&withoutInstall, "without-install", envOrDefaultValue("WITHOUT_INSTALL", false), "Execute without install")
	flag.BoolVar(&disableupdate, "disableupdate", envOrDefaultValue("DISABLEUPDATE", false), "Disable self-hosted runner automatic update to the latest released version")
	flag.Parse()

	check()
	if !withoutInstall {
		install(runnerVersion)
		if onlyInstall {
			os.Exit(0)
		}
	}

	if (repository == "" && organization == "") || (repository != "" && organization != "") {
		log.Fatal("exactly one of --repository or --organization must be specified")
	}

	var target string
	var isOrganization bool
	if organization != "" {
		target = organization
		isOrganization = true
	} else {
		target = repository
		isOrganization = false
	}

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGTERM, syscall.SIGKILL)

	if githubAppId != "" && githubAppInstallationId != "" && githubAppPrivateKey != "" {
		issuedToken, err := issueToken(githubAppId, githubAppInstallationId, githubAppPrivateKey)
		if err != nil {
			log.Fatalf("failed to issue token: %v", err)
		}
		token = *issuedToken
	}
	registrationToken := getRegistrationToken(target, token, isOrganization)

	if command := run(registrationToken, target, hostname, labels, runnerGroup, disableupdate); command != nil {
		done := make(chan struct{})
		go func() {
			if err := command.Wait(); err != nil {
				log.Printf("%+v", err)
			}
			close(done)
		}()

		select {
		case <-quit:
			if err := command.Process.Signal(syscall.SIGTERM); err != nil {
				log.Printf("%+v", err)
			}
			select {
			case <-done:
			case <-time.After(runnerExitTimeout):
			}
		case <-done:
		}
	}

	if githubAppId != "" && githubAppInstallationId != "" && githubAppPrivateKey != "" {
		issuedToken, err := issueToken(githubAppId, githubAppInstallationId, githubAppPrivateKey)
		if err != nil {
			log.Fatalf("failed to issue token: %v", err)
		}
		token = *issuedToken
	}
	removeToken := getRemoveToken(target, token, isOrganization)

	remove(removeToken)
}

func issueToken(githubAppId string, githubAppInstallationId string, githubAppPrivateKey string) (*string, error) {
	accessToken := struct {
		Token string `json:"token"`
	}{}

	err, jwtToken := signJwt(githubAppPrivateKey, githubAppId)
	if err != nil {
		return nil, xerrors.Errorf("failed to sign jwt: %w", err)
	}

	accessTokenRequest, err := http.NewRequest("POST", fmt.Sprintf("https://api.github.com/app/installations/%s/access_tokens", githubAppInstallationId), nil)
	if err != nil {
		return nil, xerrors.Errorf("failed to create request: %w", err)
	}

	accessTokenRequest.Header.Set("Accept", "application/vnd.github+json")
	accessTokenRequest.Header.Set("Authorization", fmt.Sprintf("Bearer %s", *jwtToken))
	accessTokenRequest.Header.Set("X-GitHub-Api-Version", "2022-11-28")
	accessTokenResponse, err := http.DefaultClient.Do(accessTokenRequest)
	if err != nil {
		return nil, xerrors.Errorf("failed to do request: %w", err)
	}
	defer func() {
		_ = accessTokenResponse.Body.Close()
	}()

	if accessTokenResponse.StatusCode != http.StatusCreated {
		return nil, xerrors.Errorf("failed to get access token: %d", accessTokenResponse.StatusCode)
	}

	if err := json.NewDecoder(accessTokenResponse.Body).Decode(&accessToken); err != nil {
		return nil, xerrors.Errorf("failed to decode access token: %w", err)
	}

	return &accessToken.Token, nil
}

func signJwt(privateKey string, clientId string) (error, *string) {
	block, _ := pem.Decode([]byte(privateKey))
	if block == nil {
		return xerrors.New("failed to decode private key"), nil
	}

	rsaPrivateKey, err := x509.ParsePKCS1PrivateKey(block.Bytes)
	if err != nil {
		return xerrors.Errorf("failed to parse private key: %w", err), nil
	}

	now := time.Now()
	claims := jwt.MapClaims{
		"iat": now.Unix(),
		"exp": now.Add(time.Minute * 10).Unix(),
		"iss": clientId,
	}

	token := jwt.NewWithClaims(jwt.SigningMethodRS256, claims)
	jwtToken, err := token.SignedString(rsaPrivateKey)
	if err != nil {
		return xerrors.Errorf("failed to sign token: %w", err), nil
	}
	return nil, &jwtToken
}
