set __fish_kubectl_timeout "--request-timeout=$FISH_KUBECTL_COMPLETION_TIMEOUT"

complete -c kubectl -x
complete -c kubectl -x -n '__fish_use_subcommand' -a attach -d 'Attach to a process that is already running inside an existing container.'
complete -c kubectl -x -n '__fish_use_subcommand' -a apply -d 'Apply a configuration to a resource by filename or stdin'
complete -c kubectl -x -n '__fish_use_subcommand' -a delete -d 'Delete resources by filenames, stdin, resources and names, or by resources and label selector'

complete -c kubectl -x -n '__fish_seen_subcommand_from attach' -d 'Pod' -a '(__fish_kubectl_print_resource pods)'
complete -c kubectl -x -n '__fish_seen_subcommand_from attach' -s c -l container -d 'Container name. If omitted, the first container in the pod will be chosen'
complete -c kubectl -x -n '__fish_seen_subcommand_from attach' -l pod-running-timeout -d 'The length of time (like 5s, 2m, or 3h, higher than zero) to wait until at least one pod is running'
complete -c kubectl -x -n '__fish_seen_subcommand_from attach' -s i -l stdin -d 'Pass stdin to the container'
complete -c kubectl -x -n '__fish_seen_subcommand_from attach' -s t -l tty -d 'Stdin is a TTY'

complete -c kubectl -r -n '__fish_seen_subcommand_from apply delete' -s f -l filename -d 'Filename, directory, or URL to files identifying the resource to get from a server.' -a '(__fish_complete_path)'
complete -c kubectl -r -n '__fish_seen_subcommand_from apply delete' -s k -l kustomize -d "Process a kustomization directory. This flag can't be used together with -f or -R." -a '(__fish_complete_directories)'

complete -c kubectl -x -l add-dir-header -d 'If true, adds the file directory to the header of the log messages'
complete -c kubectl -x -l alsologtostderr -d 'log to standard error as well as files'
complete -c kubectl -x -l as -d 'Username to impersonate for the operation' -a '(__fish_complete_users)'
complete -c kubectl -x -l as-group -d 'Group to impersonate for the operation' -a '(__fish_complete_groups)'
complete -c kubectl -x -l cache-dir -d 'Default cache directory' -a '(__fish_complete_directories)'
complete -c kubectl -r -l certificate-authority -d 'Path to a cert file for the certificate authority' -a '(__fish_complete_path)'
complete -c kubectl -r -l client-certificate -d 'Path to a client certificate file for TLS' -a '(__fish_complete_path)'
complete -c kubectl -r -l client-key -d 'Path to a client key file for TLS' -a '(__fish_complete_path)'
complete -c kubectl -x -l cluster -d 'The name of the kubeconfig cluster to use' -a '(__fish_kubectl config get-clusters --no-headers | awk \'{print $1}\')'
complete -c kubectl -x -l context -d 'The name of the kubeconfig context to use' -a '(__fish_kubectl config get-contexts --no-headers | awk \'{print $(NF-2)}\')'
complete -c kubectl -x -l insecure-skip-tls-verify -d 'If true, the server\'s certificate will not be checked for validity. This will make your HTTPS connections insecure'
complete -c kubectl -r -l kubeconfig -d 'Path to the kubeconfig file to use for CLI requests.' -a '(__fish_complete_path)'
complete -c kubectl -x -l log-backtrace-at -d 'when logging hits line file:N, emit a stack trace'
complete -c kubectl -r -l log-dir -d 'If non-empty, write log files in this directory' -a '(__fish_complete_path)'
complete -c kubectl -x -l log-file -d 'If non-empty, use this log file' -a '(__fish_complete_path)'
complete -c kubectl -x -l log-file-max-size -d 'Defines the maximum size a log file can grow to. Unit is megabytes. If the value is 0, the maximum file size is unlimited.'
complete -c kubectl -x -l log-flush-frequency -d 'Maximum number of seconds between log flushes'
complete -c kubectl -x -l logtostderr -d 'log to standard error instead of files'
complete -c kubectl -x -l match-server-version -d 'Require server version to match client version'
complete -c kubectl -x -s n -l namespace -d 'If present, the namespace scope for this CLI request' -a '(__fish_kubectl get namespace --no-headers | awk \'{print $1}\')'
complete -c kubectl -x -l password -d 'Password for basic authentication to the API server'
complete -c kubectl -x -l profile -d 'Name of profile to capture. One of (none|cpu|heap|goroutine|threadcreate|block|mutex)' -a 'none cpu heap goroutine threadcreate block mutex'
complete -c kubectl -r -l profile-output -d 'Name of the file to write the profile to'  -a '(__fish_complete_path)'
complete -c kubectl -x -l request-timeout -d 'The length of time to wait before giving up on a single server request. Non-zero values should contain a corresponding time unit (e.g. 1s, 2m, 3h). complete -c kubectl -x -lA value of zero means don\'t timeout requests.'
complete -c kubectl -x -s s -l server -d 'The address and port of the Kubernetes API server'
complete -c kubectl -x -l skip-headers -d 'If true, avoid header prefixes in the log messages'
complete -c kubectl -x -l skip-log-headers -d 'If true, avoid headers when opening log files'
complete -c kubectl -x -l stderrthreshold -d 'logs at or above this threshold go to stderr'
complete -c kubectl -x -l tls-server-name -d 'Server name to use for server certificate validation. If it is not provided, the hostname used to contact the server is used'
complete -c kubectl -x -l token -d 'Bearer token for authentication to the API server'
complete -c kubectl -x -l user -d 'The name of the kubeconfig user to use'
complete -c kubectl -x -l username -d 'Username for basic authentication to the API server'
complete -c kubectl -x -s v -l v -d 'number for the log level verbosity'
complete -c kubectl -x -l vmodule -d 'comma-separated list of pattern=N settings for file-filtered logging'
complete -c kubectl -x -l warnings-as-errors -d 'Treat warnings received from the server as errors and exit with a non-zero exit code'

function __fish_kubectl_print_resource -d 'Print a list of resources' -a resource
  set -l args
  if set -l ns_flags (__fish_kubectl_get_ns_flags | string split " ")
    for ns in $ns_flags
      set args $args $ns
    end
  end

  set args $args get "$resource"
  __fish_kubectl $args --no-headers 2>/dev/null | cut -d' ' -f1 | string replace -r '(.*)/' ''
end

function __fish_kubectl
	set -l context_args

	if set -l context_flags (__fish_kubectl_get_context_flags | string split ' ')
		for c in $context_flags
			set context_args $context_args $c
		end
	end

  command kubectl $__fish_kubectl_timeout $context_args $argv
end

function __fish_kubectl_get_context_flags
	set -l cmd (commandline -opc)
	if [ (count $cmd) -eq 0 ]
		return 1
	end

	set -l foundContext 0

	for c in $cmd
		test $foundContext -eq 1
		set -l out '--context' "$c"
		and echo $out
		and return 0

		if string match -q -r -- '--context=' "$c"
			set -l out (string split -- '=' "$c" | string join ' ')
			and echo $out
			and return 0
		else if contains -- "$c" '--context'
			set foundContext 1
		end
	end

	return 1
end

function __fish_kubectl_get_ns_flags
  set -l cmd (commandline -opc)
  if [ (count $cmd) -eq 0 ]
    return 1
  end

  set -l foundNamespace 0

  for c in $cmd
    test $foundNamespace -eq 1
    set -l out '--namespace' "$c"
    and echo $out
    and return 0

    if contains -- "$c" '--all-namespaces'
      echo "--all-namespaces"
      return 0
    end

    if contains -- $c "--namespace" "-n"
      set foundNamespace 1
    end
  end

  return 1
end
