#!/usr/bin/env bash

set -eo pipefail
trap 'echo "exit $?: $BASH_COMMAND(line $LINENO)" >&2' ERR

cd /var/certs
mkcert -install
mkcert 127.0.0.1
mkcert "*.127.0.0.1.nip.io"
mkcert "*.minikube.127.0.0.1.nip.io"
cat /var/certs/127.0.0.1.pem /var/certs/127.0.0.1-key.pem > /var/certs/127.0.0.1-concatenate.pem
cat /var/certs/_wildcard.127.0.0.1.nip.io.pem /var/certs/_wildcard.127.0.0.1.nip.io-key.pem > /var/certs/_wildcard.127.0.0.1.nip.io-concatenate.pem
cat /var/certs/_wildcard.minikube.127.0.0.1.nip.io.pem /var/certs/_wildcard.minikube.127.0.0.1.nip.io-key.pem > /var/certs/_wildcard.minikube.127.0.0.1.nip.io-concatenate.pem

mkdir -p ~/.ssh
chmod 700 ~/.ssh
touch ~/.ssh/config
chmod 600 ~/.ssh/config
if ! grep -q "Host github.com" ~/.ssh/config; then
  cat <<EOF >> ~/.ssh/config
Host github.com
  User git
  IdentityFile ~/.ssh/github
EOF
fi
if [ ! -f ~/.ssh/github ]; then
  ssh-keygen -t ed25519 -f ~/.ssh/github -N ""
fi
mkdir -p ~/bin
mkdir -p ~/.secrets
chmod 700 ~/.secrets
mkdir -p ~/.minikube
chattr +C ~/.minikube

if ! gpg --list-keys kaidotio@gmail.com; then
  gpg --batch --full-generate-key <(
  cat <<EOF
%no-protection
Key-Type: default
Key-Curve: ed25519
Expire-Date: 0
Name-Email: kaidotio@gmail.com
EOF
  )
  gpg --armor --export kaidotio@gmail.com > ~/.ssh/github.gpg
fi

asdf plugin add ruby https://github.com/asdf-vm/asdf-ruby.git
asdf plugin add nodejs https://github.com/asdf-vm/asdf-nodejs.git
asdf install ruby
asdf install nodejs

curl -fsSL https://sh.rustup.rs | CARGO_HOME=~/.cargo bash -s -- -y --no-modify-path
curl -fsSL https://astral.sh/uv/install.sh | CARGO_HOME=~/.cargo bash -s -- --no-modify-path
curl -fsSL https://deno.land/install.sh | bash -s -- -y --no-modify-path

curl -fsSL https://downloads.slack-edge.com/slack-cli/install.sh | sudo -E bash -s -- -d
curl -fsSL https://install.duckdb.org | bash

curl -fsSL https://dl.k8s.io/release/$(curl -L -s https://dl.k8s.io/release/stable.txt)/bin/linux/amd64/kubectl -o ~/bin/kubectl
chmod +x ~/bin/kubectl
curl -fsSL https://storage.googleapis.com/skaffold/releases/latest/skaffold-linux-amd64 -o ~/bin/skaffold
chmod +x ~/bin/skaffold
curl -fsSL https://github.com/telepresenceio/telepresence/releases/latest/download/telepresence-linux-amd64 -o ~/bin/telepresence
chmod +x ~/bin/telepresence

npm install -g chrome-devtools-mcp
npm install -g @google/clasp
npm install -g @dotenvx/dotenvx
npm install -g @google/gemini-cli
npm install -g @openai/codex
npm install -g @anthropic-ai/claude-code

rustup default nightly
rustup target add aarch64-linux-android armv7-linux-androideabi i686-linux-android x86_64-linux-android
cargo install wasm-pack
cargo install cross
cargo install typos-cli@1.42.3
cargo install cargo-expand
cargo install cargo-udeps
# Dependencies
cargo install grcov
cargo install evcxr_repl

go install sigs.k8s.io/kustomize/kustomize/v5@latest
go install sigs.k8s.io/kind@latest
go install github.com/derailed/k9s@latest

claudex mcp add -t stdio notify --scope user -- armyknife mcp notify
claudex mcp add -t stdio gemini --scope user -- armyknife mcp gemini
claudex mcp add -t stdio codex --scope user -- armyknife mcp codex
claudex mcp add -t stdio claude --scope user -- armyknife mcp claude
#claudex mcp add -t sse playwright --scope user -- http://playwright-mcp.127.0.0.1.nip.io/sse
#claudex mcp add -t sse chrome-devtools --scope user -- http://chrome-devtools-mcp.127.0.0.1.nip.io/sse
claudex mcp add -t stdio chrome-devtools --scope user -- chrome-devtools-mcp --browserUrl http://127.0.0.1:59222
claudex mcp add -t http graphiti --scope user -- http://graphiti-mcp-server.127.0.0.1.nip.io/mcp
#claudex mcp add -t http gmail --scope user -- https://gmail.mcp.claude.com/mcp
#claudex mcp add -t http gcal --scope user -- https://gcal.mcp.claude.com/mcp
