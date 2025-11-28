#!/usr/bin/env bash

set -e

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
cat <<EOF >> ~/.ssh/config
Host github.com
  User git
  IdentityFile ~/.ssh/github
EOF
ssh-keygen -t ed25519 -f ~/.ssh/github -N ""
mkdir -p ~/bin
mkdir -p ~/.secrets
chmod 700 ~/.secrets
mkdir ~/.minikube
chattr +C ~/.minikube

gpg --batch --full-generate-key <(
cat <<EOF
%no-protection
Key-Type: default
Key-Curve: ed25519
Expire-Date: 0
Name-Email: kaidotio@gmail.com
EOF
) 2>&1 | tail -n1 | awk '{print $NF}' | xargs -r basename -s .rev | xargs -r gpg --armor --export > ~/.ssh/github.gpg

asdf plugin add ruby https://github.com/asdf-vm/asdf-ruby.git
asdf plugin add nodejs https://github.com/asdf-vm/asdf-nodejs.git
asdf install

curl -fsSL https://sh.rustup.rs | CARGO_HOME=~/.cargo bash -s -- -y --no-modify-path
curl -fsSL https://astral.sh/uv/install.sh | CARGO_HOME=~/.cargo bash -s -- --no-modify-path
curl -fsSL https://deno.land/install.sh | bash

curl -fsSL https://downloads.slack-edge.com/slack-cli/install.sh | sudo -E bash -s -- -d
curl -fsSL https://install.duckdb.org | bash

curl -fsSL https://dl.k8s.io/release/$(curl -L -s https://dl.k8s.io/release/stable.txt)/bin/linux/amd64/kubectl -o ~/bin/kubectl
chmod +x ~/bin/kubectl
curl -fsSL https://storage.googleapis.com/skaffold/releases/latest/skaffold-linux-amd64 -o ~/bin/skaffold
chmod +x ~/bin/skaffold

npm install -g @google/clasp
npm install -g @dotenvx/dotenvx
npm install -g @google/gemini-cli
npm install -g @openai/codex
npm install -g @anthropic-ai/claude-code

rustup default nightly
rustup target add aarch64-linux-android armv7-linux-androideabi i686-linux-android x86_64-linux-android
cargo install wasm-pack
cargo install cross
cargo install cargo-expand
cargo install cargo-udeps
# Dependencies
cargo install grcov
cargo install evcxr_repl

go install sigs.k8s.io/kustomize/kustomize/v5@latest
go install sigs.k8s.io/kind@latest
go install github.com/derailed/k9s@latest

claude mcp add gemini armyknife mcp gemini --scope user
claude mcp add codex armyknife mcp codex --scope user
claude mcp add playwright -t sse http://playwright-mcp.127.0.0.1.nip.io/sse --scope user
claude mcp add notify armyknife mcp notify --scope user
claude mcp add notify armyknife mcp tmux --scope user
