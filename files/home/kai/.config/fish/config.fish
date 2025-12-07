if test -z $TMUX
  tmux new-session -s (pwd) 2> /dev/null || tmux attach-session -t (pwd)
else
  # https://github.com/JetBrains/jediterm/issues/148
  export TERM=ansi
end
export PYTHON_KEYRING_BACKEND=keyring.backends.null.Keyring

alias env=safenv

if ! test -e ~/.config/gcloud/configurations/config_default
  gcloud auth login
  gcloud config set project rich-gift-314809
end
test -e ~/.config/gcloud/application_default_credentials.json || gcloud auth application-default login
if ! systemctl status cloudflared > /dev/null 2>&1
  echo (set_color red)"cloudflared is not running"(set_color normal)
end
if test -z $GITHUB_TOKEN
  echo (set_color red)"GITHUB_TOKEN is not set"(set_color normal)
else
  if ! test -e ~/.docker/config.json
    echo $GITHUB_TOKEN | docker login ghcr.io -u kaidotio --password-stdin
  end

  if test -e ~/.ssh/github.pub
    set key (cat ~/.ssh/github.pub | cut -d' ' -f1,2)
    set keys (curl -fsSL -H "Accept: application/vnd.github+json" -H "Authorization: Bearer $GITHUB_TOKEN" -H "X-GitHub-Api-Version: 2022-11-28" https://api.github.com/users/kaidotio/keys)
    if test (echo $keys | jq -r '. | map(select(.key == "'$key'")) | length') -eq 0
      if curl -fsSL -H "Accept: application/vnd.github+json" -H "Authorization: Bearer $GITHUB_TOKEN" -H "X-GitHub-Api-Version: 2022-11-28" -X POST -d "{\"title\":\"$hostname\",\"key\":\"$key\"}"   https://api.github.com/user/keys
        rm ~/.ssh/github.pub
      end
    end
  end

  if test -e ~/.ssh/github.gpg
    set key (cat ~/.ssh/github.gpg | jq -Rs)
    set keys (curl -fsSL -H "Accept: application/vnd.github+json" -H "Authorization: Bearer $GITHUB_TOKEN" -H "X-GitHub-Api-Version: 2022-11-28" https://api.github.com/user/gpg_keys)
    if test (echo $keys | jq -r ". | map(select(.raw_key == $key)) | length") -eq 0
      if curl -fsSL -H "Accept: application/vnd.github+json" -H "Authorization: Bearer $GITHUB_TOKEN" -H "X-GitHub-Api-Version: 2022-11-28" -X POST -d "{\"armored_public_key\":$key}" https://api.github.com/user/gpg_keys
        git config --global user.signingkey $(gpg --with-colons --show-key --keyid-format LONG ~/.ssh/github.gpg | awk -F: '/^fpr/{print $(NF-1)}')
        rm ~/.ssh/github.gpg
      end
    end
  end
end

function __preexec --on-event fish_preexec
  history --merge
end
function __postexec --on-event fish_postexec
  history --save
end
function __exit --on-event fish_exit
  for f in ~/.secrets/*
    echo all | history --delete --contains (cat $f)
  end
end

function __fzf_reverse_isearch
  history | awk '{duplicate[$0]++}{if (duplicate[$0] == 1) print $0}' | eval fzf --tiebreak=index --height 70% -q '$(commandline)' | perl -p -e 'chomp if eof' | read -lz result && commandline -- $result
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
