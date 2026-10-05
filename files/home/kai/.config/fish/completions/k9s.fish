set __fish_kubectl_timeout "--request-timeout=$FISH_KUBECTL_COMPLETION_TIMEOUT"

complete -c k9s -x
complete -c k9s -x -n '__fish_use_subcommand' -a help -d 'Help about any command'
complete -c k9s -x -n '__fish_use_subcommand' -a info -d 'Print configuration info'
complete -c k9s -x -n '__fish_use_subcommand' -a version -d 'Print version/build info'

complete -c k9s -x -s A -l all-namespaces -d 'Launch K9s in all namespaces'
complete -c k9s -x -l as -d 'Username to impersonate for the operation' -a '(__fish_complete_users)'
complete -c k9s -x -l as-group -d 'Group to impersonate for the operation' -a '(__fish_complete_groups)'
complete -c k9s -r -l certificate-authority -d 'Path to a cert file for the certificate authority' -a '(__fish_complete_path)'
complete -c k9s -r -l client-certificate -d 'Path to a client certificate file for TLS' -a '(__fish_complete_path)'
complete -c k9s -r -l client-key -d 'Path to a client key file for TLS' -a '(__fish_complete_path)'
complete -c k9s -x -l cluster -d 'The name of the kubeconfig cluster to use' -a '(__fish_kubectl config get-clusters --no-headers | cut -d" " -f1)'
complete -c k9s -x -s c -l command -d 'Specify the default command to view when the application launches'
complete -c k9s -x -l context -d 'The name of the kubeconfig context to use' -a '(__fish_kubectl config get-contexts --no-headers | cut -d" " -f3)'
complete -c k9s -x -l headless -d 'Turn K9s header off'
complete -c k9s -x -s h -l help -d 'help for k9s'
complete -c k9s -r -l insecure-skip-tls-verify -d "If true, the server's caCertFile will not be checked for validity" -a '(__fish_complete_path)'
complete -c k9s -r -l kubeconfig -d 'Path to the kubeconfig file to use for CLI requests' -a '(__fish_complete_path)'
complete -c k9s -x -s l -l loglevel -d 'Specify a log level (info, warn, debug, error, fatal, panic, trace) (default "info")' -a 'info warn debug error fatal panic trace'
complete -c k9s -x -s n -l namespace -d 'If present, the namespace scope for this CLI request' -a '(__fish_kubectl get namespace --no-headers | cut -d" " -f1)'
complete -c k9s -x -l readonly -d 'Disable all commands that modify the cluster'
complete -c k9s -x -s r -l refresh -d 'Specify the default refresh rate as an integer (sec) (default 2)'
complete -c k9s -x -l request-timeout -d 'The length of time to wait before giving up on a single server request'
complete -c k9s -x -l token -d 'Bearer token for authentication to the API server'
complete -c k9s -x -l user -d 'The name of the kubeconfig user to use'

function __fish_kubectl
	set -l context_args

	if set -l context_flags (__fish_k9s_get_context_flags | string split ' ')
		for c in $context_flags
			set context_args $context_args $c
		end
	end

  command kubectl $__fish_kubectl_timeout $context_args $argv
end

function __fish_k9s_get_context_flags
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
