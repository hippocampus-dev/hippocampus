import * as core from "@actions/core";
import * as github from "@actions/github";
import {readFile} from "fs/promises";

interface GitHubUser {
    login: string;
}

interface GitHubTeam {
    slug: string;
}

interface GitHubFile {
    filename: string;
}

interface GitHubReview {
    id: number;
    user: GitHubUser;
    state: string;
    submitted_at?: string;
}

interface GitHubComment {
    id: number;
    body?: string;
}


interface ReviewRequestPage {
    users?: GitHubUser[];
    teams?: GitHubTeam[];
}

interface TeamMember {
    login: string;
}

class CodeownersPattern {
    pattern: string;
    isAbsolute: boolean;
    isDirectory: boolean;
    regex: RegExp;

    constructor(pattern: string) {
        this.pattern = pattern;
        this.isAbsolute = pattern.startsWith('/');
        this.isDirectory = pattern.endsWith('/');
        this.regex = this._compileRegex();
    }

    _compileRegex(): RegExp {
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
        } else {
            regexPattern = this.isDirectory ? `(^|/)${regexPattern}(/.*)?$` : `(^|/)${regexPattern}$`;
        }

        return new RegExp(regexPattern);
    }

    matches(filepath: string): boolean {
        return this.regex.test(filepath);
    }
}

class CodeownersRule {
    pattern: CodeownersPattern;
    owners: string[];

    constructor(pattern: string, owners: string[]) {
        this.pattern = new CodeownersPattern(pattern);
        this.owners = owners;
    }

    matches(filepath: string): boolean {
        return this.pattern.matches(filepath);
    }
}

class Codeowners {
    rules: CodeownersRule[];

    constructor(content: string) {
        this.rules = [];
        const lines = content.split('\n');

        for (const line of lines) {
            const trimmed = line.trim();
            if (!trimmed || trimmed.startsWith('#')) continue;

            const parts = trimmed.split(/\s+/);
            if (parts.length < 2) continue;

            const pattern = parts[0];
            const owners = parts.slice(1);
            this.rules.push(new CodeownersRule(pattern, owners));
        }
    }

    getOwners(filepath: string): string[] {
        let matchedOwners: string[] = [];

        for (const rule of this.rules) {
            if (rule.matches(filepath)) {
                matchedOwners = rule.owners;
            }
        }

        return matchedOwners;
    }
}

const teamMembersCache = new Map<string, string[]>();

const main = async (): Promise<void> => {
    const context = github.context;

    if (!context.payload.pull_request) {
        return;
    }

    const organizationToken = core.getInput('organization-token', {required: true});
    const repositoryToken = core.getInput('repository-token', {required: true});
    const ownersFiles = core.getInput('owners-files', {required: true});
    const organizationOctokit = github.getOctokit(organizationToken);
    const repositoryOctokit = github.getOctokit(repositoryToken);

    const getTeamMembers = async (organization: string, teamSlug: string): Promise<string[]> => {
        const cacheKey = `${organization}/${teamSlug}`;

        if (!teamMembersCache.has(cacheKey)) {
            const members = await organizationOctokit.paginate(organizationOctokit.rest.teams.listMembersInOrg, {
                org: organization,
                team_slug: teamSlug
            });
            teamMembersCache.set(cacheKey, members.map((m: TeamMember) => m.login));
        }

        return teamMembersCache.get(cacheKey);
    }

    const listedFiles = await repositoryOctokit.paginate(repositoryOctokit.rest.pulls.listFiles, {
        owner: context.repo.owner,
        repo: context.repo.repo,
        pull_number: context.payload.pull_request.number
    });
    const changedFiles = listedFiles.map((f: GitHubFile) => f.filename);

    const standardCodeowners = new Map<string, Codeowners>();
    for (const ownersFile of ['.github/CODEOWNERS', 'CODEOWNERS', 'docs/CODEOWNERS']) {
        try {
            const content = await readFile(ownersFile, 'utf8');
            standardCodeowners.set(ownersFile, new Codeowners(content));
            break;
        } catch {
        }
    }

    const additionalCodeowners = new Map<string, Codeowners>();
    for (const ownersFile of ownersFiles.split(',').map((f: string) => f.trim())) {
        const content = await readFile(ownersFile, 'utf8');
        additionalCodeowners.set(ownersFile, new Codeowners(content));
    }

    const standardOwnersMap = new Map<string, string[]>();
    const additionalOwnersMap = new Map<string, Map<string, string[]>>();

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
                additionalOwnersMap.get(file)!.set(ownersFile, owners);
            }
        }
    }

    if (additionalOwnersMap.size === 0) {
        core.info('No approvals required');
        return;
    }

    const allOwners = new Set();
    for (const ownersFileMap of additionalOwnersMap.values()) {
        for (const owners of ownersFileMap.values()) {
            owners.forEach((owner: string) => allOwners.add(owner));
        }
    }

    if (context.eventName === 'pull_request' && ['opened', 'synchronize'].includes(context.payload.action)) {
        const reviewRequests = await repositoryOctokit.paginate(repositoryOctokit.rest.pulls.listRequestedReviewers, {
            owner: context.repo.owner,
            repo: context.repo.repo,
            pull_number: context.payload.pull_request.number
        });

        const existingUsers = new Set(reviewRequests.flatMap((page: ReviewRequestPage) => page.users || []).map((u: GitHubUser) => u.login));
        const existingTeams = new Set(reviewRequests.flatMap((page: ReviewRequestPage) => page.teams || []).map((t: GitHubTeam) => t.slug));

        const users: string[] = [];
        const teams: string[] = [];

        for (const owner of allOwners) {
            const clean = (owner as string).startsWith('@') ? (owner as string).substring(1) : owner as string;
            const isTeam = clean.includes('/');

            if (isTeam) {
                const [organization, teamSlug] = clean.split('/');
                if (!existingTeams.has(teamSlug)) {
                    teams.push(teamSlug);
                }

                const teamMembers = await getTeamMembers(organization, teamSlug);
                teamMembers.forEach((member: string) => {
                    if (!existingUsers.has(member) && member !== context.payload.pull_request.user.login) {
                        users.push(member);
                    }
                });
            } else {
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

    const latestReviews = new Map<string, GitHubReview>();
    for (const review of allReviews) {
        const user = review.user.login;
        if (!latestReviews.has(user) ||
            new Date(review.submitted_at) > new Date(latestReviews.get(user).submitted_at)) {
            latestReviews.set(user, review);
        }
    }

    const getFileLastModifiedTime = async (file: string): Promise<Date> => {
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

    const isApprovalValidForFile = async (approval: GitHubReview, file: string): Promise<boolean> => {
        const approvalTime = new Date(approval.submitted_at);
        const fileModifiedTime = await getFileLastModifiedTime(file);
        return approvalTime >= fileModifiedTime;
    };

    const getValidApprovedUsersForFile = async (file: string): Promise<string[]> => {
        const validApprovals: string[] = [];
        for (const review of latestReviews.values()) {
            if (await isApprovalValidForFile(review, file)) {
                validApprovals.push(review.user.login);
            }
        }
        return validApprovals;
    };

    const getFileApprovalStatus = async (file: string, owners: string[]): Promise<{
        hasRequiredApproval: boolean;
        approvedOwners: string[];
        pendingOwners: string[];
        reviewsToDismiss: GitHubReview[];
    }> => {
        const validApprovedUsers = await getValidApprovedUsersForFile(file);
        const approvedOwners: string[] = [];
        const pendingOwners: string[] = [];
        const reviewsToDismiss: GitHubReview[] = [];
        const ownerUsernames = new Set<string>();

        for (const owner of owners) {
            const clean = owner.startsWith('@') ? owner.substring(1) : owner;
            const isTeam = clean.includes('/');

            if (isTeam) {
                const [organization, teamSlug] = clean.split('/');
                const teamMembers = (await getTeamMembers(organization, teamSlug)).filter((m: string) => m !== context.payload.pull_request.user.login);
                teamMembers.forEach((member: string) => ownerUsernames.add(member));

                const approver = teamMembers.find((member: string) => validApprovedUsers.includes(member));
                if (approver) {
                    approvedOwners.push(`${owner} (approved by @${approver})`);
                } else {
                    pendingOwners.push(owner);
                }
            } else {
                if (clean !== context.payload.pull_request.user.login) {
                    ownerUsernames.add(clean);

                    if (validApprovedUsers.includes(clean)) {
                        approvedOwners.push(owner);
                    } else {
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
            .filter((review: GitHubReview) => review.state === 'APPROVED' && review.user.login !== context.payload.pull_request.user.login);

        if (approvers.length === 0) {
            const message = 'No CODEOWNERS file found. At least one approval is required.';
            core.setFailed(message);
            return;
        }
    } else {
        for (const [file, owners] of standardOwnersMap.entries()) {
            const approvalStatus = await getFileApprovalStatus(file, owners);

            if (!approvalStatus.hasRequiredApproval) {
                const message = `CODEOWNERS approval missing for ${file}: needs approval from one of [${owners.join(', ')}]`;
                core.setFailed(message);
                return;
            }
        }
    }

    const header = '## :clipboard: Additional Approval Requirements\n\n';
    let commentBody = header;

    commentBody += 'This PR requires additional approvals based on the following rules:\n\n';
    commentBody += '**Important:** Approvals are tracked per-file. If a file is modified after approval, the approval for that specific file becomes invalid and needs to be renewed.\n\n';

    const filesByOwnerFile = new Map<string, Array<{
        file: string;
        owners: string[];
        approvalStatus: Awaited<ReturnType<typeof getFileApprovalStatus>>;
    }>>();
    const unapprovedFiles: Array<{
        file: string;
        missingApprovals: Array<{
            ownersFile: string;
            owners: string[];
        }>;
    }> = [];

    for (const [file, ownersFileMap] of additionalOwnersMap.entries()) {
        const missingApprovals: Array<{
            ownersFile: string;
            owners: string[];
        }> = [];

        for (const [ownersFile, owners] of ownersFileMap.entries()) {
            const approvalStatus = await getFileApprovalStatus(file, owners);

            if (!filesByOwnerFile.has(ownersFile)) {
                filesByOwnerFile.set(ownersFile, []);
            }
            filesByOwnerFile.get(ownersFile)!.push({file, owners, approvalStatus});

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

        for (const {file, owners, approvalStatus} of grouping) {
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

    const botComments = comments.filter((comment: GitHubComment) =>
        comment.body?.startsWith(header)
    );

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
        const message = unapprovedFiles.map(({file, missingApprovals}) => {
            const approvalDetails = missingApprovals.map(({ownersFile, owners}) =>
                `${ownersFile}: needs approval from one of [${owners.join(', ')}]`
            ).join('; ');
            return `${file} - ${approvalDetails}`;
        }).join('\n');
        core.setFailed(`Files missing required approvals:\n${message}`);
    }
}

main().catch((error: unknown) => {
    const message = error instanceof Error ? error.stack ?? error.message : String(error);
    core.setFailed(message);
});
