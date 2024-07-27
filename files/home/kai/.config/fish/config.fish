if test -z $TMUX
  tmux new-session -s (pwd) 2> /dev/null || tmux attach-session -t (pwd)
else
  # https://github.com/JetBrains/jediterm/issues/148
  export TERM=ansi
end
export DENO_TLS_CA_STORE=system
export PYTHON_KEYRING_BACKEND=keyring.backends.null.Keyring
export MC_HOST_minio=http://minio:miniominio@127.0.0.1:9000

if ! test -e ~/.config/gcloud/configurations/config_default
  gcloud auth login
  gcloud config set project rich-gift-314809
end
test -e ~/.config/gcloud/application_default_credentials.json || gcloud auth application-default login
if ! test -e ~/.docker/config.json
  if test -z $GITHUB_TOKEN
    echo (set_color red)"~/.docker/config.json is not found"(set_color normal)
  else
    echo $GITHUB_TOKEN | docker login ghcr.io -u kaidotio --password-stdin
  end
end

if test -e ~/.ssh/github.pub
  if ! test -z $GITHUB_TOKEN
    set key (cat ~/.ssh/github.pub | cut -d' ' -f1,2)
    set keys (curl -fsSL -H "Accept: application/vnd.github+json" -H "Authorization: Bearer $GITHUB_TOKEN" -H "X-GitHub-Api-Version: 2022-11-28" https://api.github.com/users/kaidotio/keys)
    if test (echo $keys | jq -r ". | map(select(.key == \"$key\")) | length") -eq 0
      echo (set_color red)"~/.ssh/github.pub is not registered"(set_color normal)
    else
      rm ~/.ssh/github.pub
    end
  end
end

function pre_sync --on-event fish_preexec
  history --merge
end
function post_sync --on-event fish_postexec
  history --save
end

function __fzf_reverse_isearch
  history | awk '{duplicate[$0]++}{if (duplicate[$0] == 1) print $0}' | eval fzf --tiebreak=index --height 70% -q '$(commandline)' | perl -p -e 'chomp if eof' | read -lz result; and commandline -- $result
  commandline -f repaint
end

function __fzf_kube_context
  kubectl config get-contexts -o name | eval fzf --height 70% -q '(commandline)' | perl -p -e 'chomp if eof' | read -lz result && kubectl config use-context $result
end

function fish_user_key_bindings
  bind \cr '__fzf_reverse_isearch'
  bind \cs '__fzf_kube_context'
end
funcsave fish_user_key_bindings > /dev/null

function fish_right_prompt
  set_color blue
  echo -n "["
  echo -n (kubectl config current-context 2> /dev/null)
  echo -n "]"
  set_color normal
  echo -n " "
end
