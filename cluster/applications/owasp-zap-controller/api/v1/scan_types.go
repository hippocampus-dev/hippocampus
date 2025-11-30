package v1

import (
	"fmt"
	"path/filepath"
	"strings"

	corev1 "k8s.io/api/core/v1"
	metaV1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// +kubebuilder:skipversion

// ScanSpec defines the desired state of Scan
type ScanSpec struct {
	// ZAP Automation Framework configuration (native structure)
	Automation ZAPAutomation `json:"automation" yaml:"automation"`

	// Reference to the ZAP image to use
	// +kubebuilder:default="ghcr.io/zaproxy/zaproxy:stable"
	ZapImage string `json:"zapImage,omitempty"`

	// Resources for the scan job
	Resources corev1.ResourceRequirements `json:"resources,omitempty"`
}

// ZAPAutomation represents the complete ZAP Automation Framework configuration
type ZAPAutomation struct {
	Env  ZAPEnvironment `json:"env" yaml:"env"`
	Jobs []ZAPJob       `json:"jobs" yaml:"jobs"`
}

// ReportPaths extracts the report file paths from the automation configuration
func (automation *ZAPAutomation) ReportPaths() []string {
	var paths []string
	for _, job := range automation.Jobs {
		if job.Type == "report" {
			reportFile, hasFile := job.Parameters["reportFile"]
			reportDir, hasDir := job.Parameters["reportDir"]

			if hasFile && reportFile != "" {
				if hasDir && reportDir != "" {
					paths = append(paths, filepath.Join(reportDir, reportFile))
				} else {
					paths = append(paths, reportFile)
				}
			}
		}
	}
	return paths
}

// InjectCredentialPlaceholders injects environment variable placeholders for credentials
func (automation *ZAPAutomation) InjectCredentialPlaceholders() {
	for i, ctx := range automation.Env.Contexts {
		if ctx.Authentication != nil && ctx.Authentication.Credentials != nil {
			if ctx.Authentication.Parameters == nil {
				ctx.Authentication.Parameters = make(map[string]string)
			}

			for secretKey, envVarName := range ctx.Authentication.Credentials.EnvMappings {
				placeholder := fmt.Sprintf("{%%%s%%}", secretKey)
				envRef := fmt.Sprintf("${%s}", envVarName)

				switch ctx.Authentication.Method {
				case "http", "form", "json":
					ctx.Authentication.Parameters[secretKey] = envRef
				case "browser":
					if browserScript, exists := ctx.Authentication.Parameters["browserScript"]; exists {
						ctx.Authentication.Parameters["browserScript"] = strings.ReplaceAll(browserScript, placeholder, envRef)
					}
				case "script":
					if scriptParams := ctx.Authentication.Parameters["scriptParameters"]; scriptParams != "" {
						ctx.Authentication.Parameters["scriptParameters"] = strings.ReplaceAll(scriptParams, placeholder, envRef)
					}
					if scriptInline, exists := ctx.Authentication.Parameters["scriptInline"]; exists {
						ctx.Authentication.Parameters["scriptInline"] = strings.ReplaceAll(scriptInline, placeholder, envRef)
					}
				case "manual":
					// No credential injection needed
				}
			}
		}

		for j, user := range ctx.Users {
			if user.CredentialsRef != nil {
				userNameUpper := strings.ToUpper(strings.ReplaceAll(user.Name, "-", "_"))
				automation.Env.Contexts[i].Users[j].Username = fmt.Sprintf("${ZAP_USER_%s_USERNAME}", userNameUpper)
				automation.Env.Contexts[i].Users[j].Password = fmt.Sprintf("${ZAP_USER_%s_PASSWORD}", userNameUpper)
			}
		}
	}
}

// CollectSecretNames collects all unique secret names referenced in the automation configuration
func (automation *ZAPAutomation) CollectSecretNames() []string {
	secretMap := make(map[string]bool)

	for _, ctx := range automation.Env.Contexts {
		if ctx.Authentication != nil && ctx.Authentication.Credentials != nil {
			secretMap[ctx.Authentication.Credentials.SecretRef.Name] = true
		}

		for _, user := range ctx.Users {
			if user.CredentialsRef != nil {
				secretMap[user.CredentialsRef.Name] = true
			}
		}
	}

	var secrets []string
	for secretName := range secretMap {
		secrets = append(secrets, secretName)
	}

	return secrets
}

// BuildEnvVars builds the environment variables for credential injection
func (automation *ZAPAutomation) BuildEnvVars() []corev1.EnvVar {
	var envVars []corev1.EnvVar

	for _, ctx := range automation.Env.Contexts {
		if ctx.Authentication != nil && ctx.Authentication.Credentials != nil {
			secretName := ctx.Authentication.Credentials.SecretRef.Name

			if ctx.Authentication.Method == "manual" {
				continue
			}

			for secretKey, envVarName := range ctx.Authentication.Credentials.EnvMappings {
				envVar := corev1.EnvVar{
					Name: envVarName,
					ValueFrom: &corev1.EnvVarSource{
						SecretKeyRef: &corev1.SecretKeySelector{
							LocalObjectReference: corev1.LocalObjectReference{
								Name: secretName,
							},
							Key: secretKey,
						},
					},
				}
				envVars = append(envVars, envVar)
			}
		}

		for _, user := range ctx.Users {
			if user.CredentialsRef != nil {
				secretName := user.CredentialsRef.Name
				userNameUpper := strings.ToUpper(strings.ReplaceAll(user.Name, "-", "_"))

				envVars = append(envVars,
					corev1.EnvVar{
						Name: fmt.Sprintf("ZAP_USER_%s_USERNAME", userNameUpper),
						ValueFrom: &corev1.EnvVarSource{
							SecretKeyRef: &corev1.SecretKeySelector{
								LocalObjectReference: corev1.LocalObjectReference{
									Name: secretName,
								},
								Key: "username",
							},
						},
					},
					corev1.EnvVar{
						Name: fmt.Sprintf("ZAP_USER_%s_PASSWORD", userNameUpper),
						ValueFrom: &corev1.EnvVarSource{
							SecretKeyRef: &corev1.SecretKeySelector{
								LocalObjectReference: corev1.LocalObjectReference{
									Name: secretName,
								},
								Key: "password",
							},
						},
					},
				)
			}
		}
	}

	return envVars
}

// ZAPEnvironment represents the environment configuration for ZAP automation
type ZAPEnvironment struct {
	// Contexts for the scan
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:MinItems=1
	Contexts []ZAPContext `json:"contexts" yaml:"contexts"`

	// +kubebuilder:default=true
	FailOnError bool `json:"failOnError,omitempty" yaml:"failOnError,omitempty"`
	// +kubebuilder:default=false
	FailOnWarning bool `json:"failOnWarning,omitempty" yaml:"failOnWarning,omitempty"`
	// +kubebuilder:default=true
	ContinueOnFailure bool `json:"continueOnFailure,omitempty" yaml:"continueOnFailure,omitempty"`
	// +kubebuilder:default=true
	ProgressToStdout bool `json:"progressToStdout,omitempty" yaml:"progressToStdout,omitempty"`

	// Environment variables
	Vars map[string]string `json:"vars,omitempty" yaml:"vars,omitempty"`
	// Parameters for the automation plan
	Parameters map[string]string `json:"parameters,omitempty" yaml:"parameters,omitempty"`
}

// ZAPContext represents a scanning context in ZAP
type ZAPContext struct {
	Name         string   `json:"name" yaml:"name"`
	URLs         []string `json:"urls" yaml:"urls"`
	IncludePaths []string `json:"includePaths,omitempty" yaml:"includePaths,omitempty"`
	ExcludePaths []string `json:"excludePaths,omitempty" yaml:"excludePaths,omitempty"`

	// Authentication configuration
	Authentication *ZAPAuthentication `json:"authentication,omitempty" yaml:"authentication,omitempty"`

	// Session management configuration
	SessionManagement *ZAPSessionManagement `json:"sessionManagement,omitempty" yaml:"sessionManagement,omitempty"`

	// Technology configuration
	Technology *ZAPTechnology `json:"technology,omitempty" yaml:"technology,omitempty"`

	// Users for authenticated scanning
	Users []ZAPUser `json:"users,omitempty" yaml:"users,omitempty"`
}

// ZAPAuthentication represents authentication configuration in ZAP
type ZAPAuthentication struct {
	// +kubebuilder:validation:Enum=http;form;json;manual;script;browser
	Method string `json:"method" yaml:"method"`

	// Authentication parameters (varies by method)
	// Using map[string]string to avoid controller-gen issues with interface{}
	Parameters map[string]string `json:"parameters,omitempty" yaml:"parameters,omitempty"`

	// Verification configuration
	Verification *ZAPVerification `json:"verification,omitempty" yaml:"verification,omitempty"`

	// Kubernetes-specific: Reference to secret containing credentials
	// This field is not included in the YAML output to ZAP
	Credentials *ZAPCredentials `json:"credentials,omitempty" yaml:"-"`
}

// ZAPCredentials represents credential configuration
type ZAPCredentials struct {
	// Reference to the Kubernetes secret
	SecretRef corev1.SecretReference `json:"secretRef" yaml:"-"`

	// Environment variable mappings from secret keys
	// Map key is the secret key, value is the environment variable name
	// Example: {"username": "MY_APP_USER", "password": "MY_APP_PASS"}
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:MinProperties=1
	EnvMappings map[string]string `json:"envMappings" yaml:"-"`
}

// ZAPVerification represents authentication verification configuration
type ZAPVerification struct {
	// +kubebuilder:validation:Enum=response;request;both;poll
	Method         string `json:"method" yaml:"method"`
	LoggedInRegex  string `json:"loggedInRegex,omitempty" yaml:"loggedInRegex,omitempty"`
	LoggedOutRegex string `json:"loggedOutRegex,omitempty" yaml:"loggedOutRegex,omitempty"`
	// Poll configuration
	PollFrequency int    `json:"pollFrequency,omitempty" yaml:"pollFrequency,omitempty"`
	PollUnits     string `json:"pollUnits,omitempty" yaml:"pollUnits,omitempty"`
	PollURL       string `json:"pollUrl,omitempty" yaml:"pollUrl,omitempty"`
	PollData      string `json:"pollData,omitempty" yaml:"pollData,omitempty"`
	PollHeaders   string `json:"pollHeaders,omitempty" yaml:"pollHeaders,omitempty"`
}

// ZAPUser represents a user for authenticated scanning
type ZAPUser struct {
	Name     string `json:"name" yaml:"name"`
	Username string `json:"username,omitempty" yaml:"username,omitempty"`
	Password string `json:"password,omitempty" yaml:"password,omitempty"`

	// Kubernetes-specific: Reference to secret containing credentials
	// This field is not included in the YAML output to ZAP
	CredentialsRef *corev1.SecretReference `json:"credentialsRef,omitempty" yaml:"-"`
}

// ZAPSessionManagement represents session management configuration
type ZAPSessionManagement struct {
	// +kubebuilder:validation:Enum=cookie;http;script
	Method string `json:"method" yaml:"method"`
	// Parameters for session management (varies by method)
	// Using map[string]string to avoid controller-gen issues with interface{}
	Parameters map[string]string `json:"parameters,omitempty" yaml:"parameters,omitempty"`
}

// ZAPTechnology represents technology inclusion/exclusion configuration
type ZAPTechnology struct {
	Include []string `json:"include,omitempty" yaml:"include,omitempty"`
	Exclude []string `json:"exclude,omitempty" yaml:"exclude,omitempty"`
}

// ZAPJob represents a job in the ZAP automation plan
type ZAPJob struct {
	// Job type (e.g., spider, spiderAjax, passiveScan-config, passiveScan-wait, activeScan, report)
	Type string `json:"type" yaml:"type"`

	// Job-specific parameters
	// Using map[string]string to avoid controller-gen issues with interface{}
	Parameters map[string]string `json:"parameters,omitempty" yaml:"parameters,omitempty"`

	// Job-specific configuration
	// Using map[string]string to avoid controller-gen issues with interface{}
	Configuration map[string]string `json:"configuration,omitempty" yaml:"configuration,omitempty"`

	// Tests to run after the job
	Tests []ZAPTest `json:"tests,omitempty" yaml:"tests,omitempty"`
}

// ZAPTest represents a test in a ZAP job
type ZAPTest struct {
	Type string `json:"type" yaml:"type"`
	// Using map[string]string to avoid controller-gen issues with interface{}
	Parameters map[string]string `json:"parameters,omitempty" yaml:"parameters,omitempty"`
}

// ScanStatus defines the observed state of Scan
type ScanStatus struct {
	// Reports contains the scan report data
	Reports map[string]string `json:"reports,omitempty"`

	// LastScanTime is the time when the last scan was triggered
	LastScanTime *metaV1.Time `json:"lastScanTime,omitempty"`

	// ObservedGeneration represents the .metadata.generation that the status was updated for
	// +optional
	ObservedGeneration int64 `json:"observedGeneration,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:printcolumn:name="LastScan",type="date",JSONPath=".status.lastScanTime"
// +kubebuilder:printcolumn:name="Target",type="string",JSONPath=".spec.automation.env.contexts[0].urls[0]"
// +kubebuilder:printcolumn:name="Age",type="date",JSONPath=".metadata.creationTimestamp"

// Scan is the schema for the scans API
type Scan struct {
	metaV1.TypeMeta   `json:",inline"`
	metaV1.ObjectMeta `json:"metadata,omitempty"`

	Spec   ScanSpec   `json:"spec,omitempty"`
	Status ScanStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// ScanList contains a list of Attack
type ScanList struct {
	metaV1.TypeMeta `json:",inline"`
	metaV1.ListMeta `json:"metadata,omitempty"`
	Items           []Scan `json:"items"`
}
