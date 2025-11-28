'use strict';

var core = require('@actions/core');
var github = require('@actions/github');
var promises = require('fs/promises');

function _interopNamespaceDefault(e) {
    var n = Object.create(null);
    if (e) {
        Object.keys(e).forEach(function (k) {
            if (k !== 'default') {
                var d = Object.getOwnPropertyDescriptor(e, k);
                Object.defineProperty(n, k, d.get ? d : {
                    enumerable: true,
                    get: function () { return e[k]; }
                });
            }
        });
    }
    n.default = e;
    return Object.freeze(n);
}

var core__namespace = /*#__PURE__*/_interopNamespaceDefault(core);
var github__namespace = /*#__PURE__*/_interopNamespaceDefault(github);

class CodeownersPattern {
    pattern;
    isAbsolute;
    isDirectory;
    regex;
    constructor(pattern) {
        this.pattern = pattern;
        this.isAbsolute = pattern.startsWith('/');
        this.isDirectory = pattern.endsWith('/');
        this.regex = this._compileRegex();
    }
    _compileRegex() {
        let normalizedPattern = this.pattern;
        if (this.isAbsolute) {
            normalizedPattern = normalizedPattern.substring(1);
        }
        if (this.isDirectory) {
            normalizedPattern = normalizedPattern.substring(0, normalizedPattern.length - 1);
        }
        let regexPattern = normalizedPattern
            .replace(/[.+^${}()|[\]\\]/g, '\\$&')
            .replace(/\\\*/g, '__STAR__')
            .replace(/__STAR____STAR__/g, '__DOUBLESTAR__')
            .replace(/__STAR__/g, '[^/]*')
            .replace(/__DOUBLESTAR__/g, '.*');
        if (this.isAbsolute) {
            regexPattern = this.isDirectory ? `^${regexPattern}(/.*)?$` : `^${regexPattern}$`;
        }
        else {
            regexPattern = this.isDirectory ? `(^|/)${regexPattern}(/.*)?$` : `(^|/)${regexPattern}$`;
        }
        return new RegExp(regexPattern);
    }
    matches(filepath) {
        return this.regex.test(filepath);
    }
}
class CodeownersRule {
    pattern;
    owners;
    constructor(pattern, owners) {
        this.pattern = new CodeownersPattern(pattern);
        this.owners = owners;
    }
    matches(filepath) {
        return this.pattern.matches(filepath);
    }
}
class Codeowners {
    rules;
    constructor(content) {
        this.rules = [];
        const lines = content.split('\n');
        for (const line of lines) {
            const trimmed = line.trim();
            if (!trimmed || trimmed.startsWith('#'))
                continue;
            const parts = trimmed.split(/\s+/);
            if (parts.length < 2)
                continue;
            const pattern = parts[0];
            const owners = parts.slice(1);
            this.rules.push(new CodeownersRule(pattern, owners));
        }
    }
    getOwners(filepath) {
        let matchedOwners = [];
        for (const rule of this.rules) {
            if (rule.matches(filepath)) {
                matchedOwners = rule.owners;
            }
        }
        return matchedOwners;
    }
}
const teamMembersCache = new Map();
const main = async () => {
    const context = github__namespace.context;
    if (!context.payload.pull_request) {
        return;
    }
    const organizationToken = core__namespace.getInput('organization-token', { required: true });
    const repositoryToken = core__namespace.getInput('repository-token', { required: true });
    const ownersFiles = core__namespace.getInput('owners-files', { required: true });
    const organizationOctokit = github__namespace.getOctokit(organizationToken);
    const repositoryOctokit = github__namespace.getOctokit(repositoryToken);
    const getTeamMembers = async (organization, teamSlug) => {
        const cacheKey = `${organization}/${teamSlug}`;
        if (!teamMembersCache.has(cacheKey)) {
            const members = await organizationOctokit.paginate(organizationOctokit.rest.teams.listMembersInOrg, {
                org: organization,
                team_slug: teamSlug
            });
            teamMembersCache.set(cacheKey, members.map((m) => m.login));
        }
        return teamMembersCache.get(cacheKey);
    };
    const listedFiles = await repositoryOctokit.paginate(repositoryOctokit.rest.pulls.listFiles, {
        owner: context.repo.owner,
        repo: context.repo.repo,
        pull_number: context.payload.pull_request.number
    });
    const changedFiles = listedFiles.map((f) => f.filename);
    const standardCodeowners = new Map();
    for (const ownersFile of ['.github/CODEOWNERS', 'CODEOWNERS', 'docs/CODEOWNERS']) {
        try {
            const content = await promises.readFile(ownersFile, 'utf8');
            standardCodeowners.set(ownersFile, new Codeowners(content));
            break;
        }
        catch {
        }
    }
    const additionalCodeowners = new Map();
    for (const ownersFile of ownersFiles.split(',').map((f) => f.trim())) {
        const content = await promises.readFile(ownersFile, 'utf8');
        additionalCodeowners.set(ownersFile, new Codeowners(content));
    }
    const standardOwnersMap = new Map();
    const additionalOwnersMap = new Map();
    for (const file of changedFiles) {
        for (const [_, codeowners] of standardCodeowners.entries()) {
            const owners = codeowners.getOwners(file);
            if (owners && owners.length > 0) {
                standardOwnersMap.set(file, owners);
            }
        }
        for (const [ownersFile, codeowners] of additionalCodeowners.entries()) {
            const owners = codeowners.getOwners(file);
            if (owners && owners.length > 0) {
                if (!additionalOwnersMap.has(file)) {
                    additionalOwnersMap.set(file, new Map());
                }
                additionalOwnersMap.get(file).set(ownersFile, owners);
            }
        }
    }
    if (additionalOwnersMap.size === 0) {
        core__namespace.info('No approvals required');
        return;
    }
    const allOwners = new Set();
    for (const ownersFileMap of additionalOwnersMap.values()) {
        for (const owners of ownersFileMap.values()) {
            owners.forEach((owner) => allOwners.add(owner));
        }
    }
    if (context.eventName === 'pull_request' && ['opened', 'synchronize'].includes(context.payload.action)) {
        const reviewRequests = await repositoryOctokit.paginate(repositoryOctokit.rest.pulls.listRequestedReviewers, {
            owner: context.repo.owner,
            repo: context.repo.repo,
            pull_number: context.payload.pull_request.number
        });
        const existingUsers = new Set(reviewRequests.flatMap((page) => page.users || []).map((u) => u.login));
        const existingTeams = new Set(reviewRequests.flatMap((page) => page.teams || []).map((t) => t.slug));
        const users = [];
        const teams = [];
        for (const owner of allOwners) {
            const clean = owner.startsWith('@') ? owner.substring(1) : owner;
            const isTeam = clean.includes('/');
            if (isTeam) {
                const [organization, teamSlug] = clean.split('/');
                if (!existingTeams.has(teamSlug)) {
                    teams.push(teamSlug);
                }
                const teamMembers = await getTeamMembers(organization, teamSlug);
                teamMembers.forEach((member) => {
                    if (!existingUsers.has(member) && member !== context.payload.pull_request.user.login) {
                        users.push(member);
                    }
                });
            }
            else {
                if (!existingUsers.has(clean) && clean !== context.payload.pull_request.user.login) {
                    users.push(clean);
                }
            }
        }
        if (users.length > 0 || teams.length > 0) {
            await repositoryOctokit.rest.pulls.requestReviewers({
                owner: context.repo.owner,
                repo: context.repo.repo,
                pull_number: context.payload.pull_request.number,
                reviewers: users,
            });
        }
    }
    const allReviews = await repositoryOctokit.paginate(repositoryOctokit.rest.pulls.listReviews, {
        owner: context.repo.owner,
        repo: context.repo.repo,
        pull_number: context.payload.pull_request.number
    });
    const latestReviews = new Map();
    for (const review of allReviews) {
        const user = review.user.login;
        if (!latestReviews.has(user) ||
            new Date(review.submitted_at) > new Date(latestReviews.get(user).submitted_at)) {
            latestReviews.set(user, review);
        }
    }
    const getFileLastModifiedTime = async (file) => {
        const commits = await repositoryOctokit.paginate(repositoryOctokit.rest.repos.listCommits, {
            owner: context.repo.owner,
            repo: context.repo.repo,
            path: file,
            sha: context.payload.pull_request.head.sha,
            per_page: 1
        });
        if (commits.length > 0 && commits[0].commit.committer.date) {
            return new Date(commits[0].commit.committer.date);
        }
        return new Date();
    };
    const isApprovalValidForFile = async (approval, file) => {
        const approvalTime = new Date(approval.submitted_at);
        const fileModifiedTime = await getFileLastModifiedTime(file);
        return approvalTime >= fileModifiedTime;
    };
    const getValidApprovedUsersForFile = async (file) => {
        const validApprovals = [];
        for (const review of latestReviews.values()) {
            if (await isApprovalValidForFile(review, file)) {
                validApprovals.push(review.user.login);
            }
        }
        return validApprovals;
    };
    const getFileApprovalStatus = async (file, owners) => {
        const validApprovedUsers = await getValidApprovedUsersForFile(file);
        const approvedOwners = [];
        const pendingOwners = [];
        const reviewsToDismiss = [];
        const ownerUsernames = new Set();
        for (const owner of owners) {
            const clean = owner.startsWith('@') ? owner.substring(1) : owner;
            const isTeam = clean.includes('/');
            if (isTeam) {
                const [organization, teamSlug] = clean.split('/');
                const teamMembers = (await getTeamMembers(organization, teamSlug)).filter((m) => m !== context.payload.pull_request.user.login);
                teamMembers.forEach((member) => ownerUsernames.add(member));
                const approver = teamMembers.find((member) => validApprovedUsers.includes(member));
                if (approver) {
                    approvedOwners.push(`${owner} (approved by @${approver})`);
                }
                else {
                    pendingOwners.push(owner);
                }
            }
            else {
                if (clean !== context.payload.pull_request.user.login) {
                    ownerUsernames.add(clean);
                    if (validApprovedUsers.includes(clean)) {
                        approvedOwners.push(owner);
                    }
                    else {
                        pendingOwners.push(owner);
                    }
                }
            }
        }
        if (approvedOwners.length === 0) {
            for (const review of latestReviews.values()) {
                if (review.state === 'APPROVED' && ownerUsernames.has(review.user.login)) {
                    reviewsToDismiss.push(review);
                }
            }
        }
        return {
            hasRequiredApproval: approvedOwners.length > 0,
            approvedOwners,
            pendingOwners,
            reviewsToDismiss
        };
    };
    if (standardCodeowners.size === 0) {
        const approvers = Array.from(latestReviews.values())
            .filter((review) => review.state === 'APPROVED' && review.user.login !== context.payload.pull_request.user.login);
        if (approvers.length === 0) {
            const message = 'No CODEOWNERS file found. At least one approval is required.';
            core__namespace.setFailed(message);
            return;
        }
    }
    else {
        for (const [file, owners] of standardOwnersMap.entries()) {
            const approvalStatus = await getFileApprovalStatus(file, owners);
            if (!approvalStatus.hasRequiredApproval) {
                const message = `CODEOWNERS approval missing for ${file}: needs approval from one of [${owners.join(', ')}]`;
                core__namespace.setFailed(message);
                return;
            }
        }
    }
    const header = '## :clipboard: Additional Approval Requirements\n\n';
    let commentBody = header;
    commentBody += 'This PR requires additional approvals based on the following rules:\n\n';
    commentBody += '**Important:** Approvals are tracked per-file. If a file is modified after approval, the approval for that specific file becomes invalid and needs to be renewed.\n\n';
    const filesByOwnerFile = new Map();
    const unapprovedFiles = [];
    for (const [file, ownersFileMap] of additionalOwnersMap.entries()) {
        const missingApprovals = [];
        for (const [ownersFile, owners] of ownersFileMap.entries()) {
            const approvalStatus = await getFileApprovalStatus(file, owners);
            if (!filesByOwnerFile.has(ownersFile)) {
                filesByOwnerFile.set(ownersFile, []);
            }
            filesByOwnerFile.get(ownersFile).push({ file, owners, approvalStatus });
            if (!approvalStatus.hasRequiredApproval) {
                missingApprovals.push({
                    ownersFile,
                    owners
                });
            }
        }
        if (missingApprovals.length > 0) {
            unapprovedFiles.push({
                file,
                missingApprovals
            });
        }
    }
    for (const [ownersFile, grouping] of filesByOwnerFile.entries()) {
        const normalizedPath = ownersFile.replace(/^\.\//, '').replace(/^\//, '');
        const fileUrl = `https://github.com/${context.repo.owner}/${context.repo.repo}/blob/${context.payload.pull_request.head.ref}/${normalizedPath}`;
        commentBody += `### [${ownersFile}](${fileUrl})\n\n`;
        commentBody += '| File | Status | Required Reviewers | Approved | Pending |\n';
        commentBody += '|------|--------|-------------------|----------|----------|\n';
        for (const { file, owners, approvalStatus } of grouping) {
            const statusIcon = approvalStatus.hasRequiredApproval ? ':white_check_mark:' : ':hourglass_flowing_sand:';
            const statusText = approvalStatus.hasRequiredApproval ? 'Approved' : 'Pending';
            for (const review of approvalStatus.reviewsToDismiss) {
                await repositoryOctokit.rest.pulls.dismissReview({
                    owner: context.repo.owner,
                    repo: context.repo.repo,
                    pull_number: context.payload.pull_request.number,
                    review_id: review.id,
                    message: `Review dismissed: File ${file} was modified after approval`
                });
            }
            const approvedText = approvalStatus.approvedOwners.length > 0
                ? approvalStatus.approvedOwners.join(', ')
                : '-';
            const pendingText = approvalStatus.hasRequiredApproval
                ? '-'
                : (approvalStatus.pendingOwners.length > 0
                    ? approvalStatus.pendingOwners.join(' OR ')
                    : '-');
            const fileDiffUrl = `https://github.com/${context.repo.owner}/${context.repo.repo}/pull/${context.payload.pull_request.number}/files#diff-${Buffer.from(file).toString('hex')}`;
            commentBody += `| [${file}](${fileDiffUrl}) | ${statusIcon} ${statusText} | ${owners.join(' OR ')} | ${approvedText} | ${pendingText} |\n`;
        }
        commentBody += '\n';
    }
    commentBody += '*Review requests have been automatically sent to all required reviewers.*\n';
    const comments = await repositoryOctokit.paginate(repositoryOctokit.rest.issues.listComments, {
        owner: context.repo.owner,
        repo: context.repo.repo,
        issue_number: context.payload.pull_request.number
    });
    const botComments = comments.filter((comment) => comment.body?.startsWith(header));
    for (const comment of botComments) {
        await repositoryOctokit.rest.issues.deleteComment({
            owner: context.repo.owner,
            repo: context.repo.repo,
            comment_id: comment.id
        });
    }
    await repositoryOctokit.rest.issues.createComment({
        owner: context.repo.owner,
        repo: context.repo.repo,
        issue_number: context.payload.pull_request.number,
        body: commentBody
    });
    if (unapprovedFiles.length > 0) {
        const message = unapprovedFiles.map(({ file, missingApprovals }) => {
            const approvalDetails = missingApprovals.map(({ ownersFile, owners }) => `${ownersFile}: needs approval from one of [${owners.join(', ')}]`).join('; ');
            return `${file} - ${approvalDetails}`;
        }).join('\n');
        core__namespace.setFailed(`Files missing required approvals:\n${message}`);
    }
};
main().catch((error) => {
    core__namespace.setFailed(`Error: ${error.message}`);
});
