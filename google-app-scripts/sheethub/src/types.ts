export interface GitHubFile {
    type: string;
    encoding: string;
    size: number;
    name: string;
    path: string;
    content: string;
    sha: string;
    url: string;
    git_url: string;
    html_url: string;
    download_url: string;
    _links: GitHubLinks;
}

interface GitHubLinks {
    git: string;
    self: string;
    html: string;
}

export interface GitHubRef {
    ref: string;
    url: string;
    object: GitHubObject;
}

interface GitHubObject {
    sha: string;
    type: string;
    url: string;
}

export interface GitHubBlob {
    url: string;
    sha: string;
}

export interface GitHubTrees {
    sha: string;
    url: string;
    tree: GitHubTree[];
    truncated: boolean;
}

interface GitHubTree {
    path: string;
    mode: string;
    type: string;
    sha: string;
    size: number;
    url: string;
}

export interface GitHubCommit {
    sha: string;
    node_id: string;
    url: string;
    author: GitHubAuthor;
    committer: GitHubCommitter;
    message: string;
    tree: GitHubTree;
    parents: GitHubParent[];
    verification: GitHubVerification;
    html_url: string;
}

interface GitHubAuthor {
    date: string;
    name: string;
    email: string;
}

interface GitHubCommitter {
    date: string;
    name: string;
    email: string;
}

interface GitHubParent {
    url: string;
    sha: string;
    html_url: string;
}

interface GitHubVerification {
    verified: boolean;
    reason: string;
    signature: any;
    payload: any;
}
