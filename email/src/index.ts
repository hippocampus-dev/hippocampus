import PostalMime from "postal-mime";
import {EmailMessage} from "cloudflare:email";
import {createMimeMessage} from "mimetext";

interface Environment {
    AI: Ai;
    SEND_EMAIL: SendEmail;
}

const ALLOWED_SENDER = "kaidotio@gmail.com";
const TRUSTED_AUTHSERV_ID = "mx.cloudflare.net";
const MAX_BODY_LENGTH = 100000;

interface EmailAuthenticationResult {
    spfPass: boolean;
    dkimPass: boolean;
}

// RFC 5322: Strip comments and folding whitespace, handling quotes and escapes
function stripCFWS(input: string): string | null {
    const MAX_HEADER_LENGTH = 8192;
    if (input.length > MAX_HEADER_LENGTH) {
        return null;
    }

    let result = "";
    let depth = 0;
    let inQuote = false;
    let escaped = false;
    let prevWasSpace = false;

    for (const char of input) {
        if (escaped) {
            if (depth === 0) {
                result += char;
                prevWasSpace = false;
            }
            escaped = false;
            continue;
        }

        if (char === "\\") {
            escaped = true;
            if (depth === 0) {
                result += char;
                prevWasSpace = false;
            }
            continue;
        }

        if (char === '"' && depth === 0) {
            inQuote = !inQuote;
            result += char;
            prevWasSpace = false;
            continue;
        }

        if (!inQuote) {
            if (char === "(") {
                if (depth === 0 && !prevWasSpace) {
                    result += " ";
                    prevWasSpace = true;
                }
                depth++;
                continue;
            }
            if (char === ")") {
                if (depth === 0) {
                    return null;
                }
                depth--;
                continue;
            }
        }

        if (depth === 0) {
            if (inQuote) {
                result += char;
                prevWasSpace = false;
            } else if (/\s/.test(char)) {
                if (!prevWasSpace) {
                    result += " ";
                    prevWasSpace = true;
                }
            } else {
                result += char;
                prevWasSpace = false;
            }
        }
    }

    if (depth !== 0 || inQuote || escaped) {
        return null;
    }

    return result.trim();
}

function extractTrustedSection(header: string): string | null {
    const stripped = stripCFWS(header);
    if (stripped === null) {
        return null;
    }

    let mostRecentHeader = stripped;
    let inQuote = false;
    let escaped = false;
    for (let i = 0; i < stripped.length; i++) {
        const char = stripped[i];
        if (escaped) {
            escaped = false;
            continue;
        }
        if (char === "\\") {
            escaped = true;
            continue;
        }
        if (char === '"') {
            inQuote = !inQuote;
        } else if (char === "," && !inQuote) {
            mostRecentHeader = stripped.slice(0, i).trim();
            break;
        }
    }

    // RFC 8617 (ARC): "i=1; authserv-id; method=result; ..."
    const normalized = mostRecentHeader.replace(/^i\s*=\s*\d+\s*;\s*/i, "").trimStart().toLowerCase();

    if (!normalized.startsWith(TRUSTED_AUTHSERV_ID)) {
        return null;
    }

    const afterAuthservId = normalized.slice(TRUSTED_AUTHSERV_ID.length).trimStart();
    // RFC 8601: authserv-id [CFWS authres-version] ";"
    const afterVersion = afterAuthservId.replace(/^\d+\s*/, "");
    if (!afterVersion.startsWith(";")) {
        return null;
    }

    return afterVersion.slice(1);
}

function verifyEmailAuthentication(
    message: ForwardableEmailMessage
): EmailAuthenticationResult {
    const header =
        message.headers.get("Authentication-Results") ??
        message.headers.get("Arc-Authentication-Results") ??
        "";

    const trustedSection = extractTrustedSection(header);
    if (trustedSection !== null) {
        return {
            spfPass: /\bspf\s*=\s*pass\b/i.test(trustedSection),
            dkimPass: /\bdkim\s*=\s*pass\b/i.test(trustedSection),
        };
    }

    // Cloudflare Email Routing enforces SPF/DKIM authentication since 2025-07-03.
    // Unauthenticated emails are rejected before reaching this Worker.
    // https://developers.cloudflare.com/changelog/2025-06-30-mail-authentication/
    return {spfPass: true, dkimPass: true};
}

function extractEmailAddress(from: string): string {
    const match = from.match(/<([^>]+)>/);
    return (match ? match[1] : from).toLowerCase().trim();
}

interface Message {
    role: "user" | "assistant";
    content: string;
}

function formatQuotedText(text: string): string {
    return text
        .split(/\r?\n/)
        .map((line) => `> ${line}`)
        .join("\n");
}

function parseQuotedText(body: string): Message[] {
    const lines = body.split(/\r?\n/);

    const groups: {depth: number; lines: string[]}[] = [];
    let currentGroup: {depth: number; lines: string[]} | null = null;

    for (const line of lines) {
        // Count quote level: handles both ">>" and "> > " formats (RFC 3676)
        const match = line.match(/^(?:>\s*)+/);
        const depth = match ? (match[0].match(/>/g) ?? []).length : 0;
        const content = match ? line.slice(match[0].length) : line;

        if (currentGroup === null || currentGroup.depth !== depth) {
            if (currentGroup !== null) {
                groups.push(currentGroup);
            }
            currentGroup = {depth, lines: [content]};
        } else {
            currentGroup.lines.push(content);
        }
    }
    if (currentGroup !== null) {
        groups.push(currentGroup);
    }

    const processedGroups = groups
        .map((group) => {
            let text = group.lines.join("\n");

            // RFC 3676: signature delimiter
            const signatureIndex = text.search(/\n-- (\n|$)/);
            if (signatureIndex !== -1) {
                text = text.slice(0, signatureIndex);
            }

            text = text.replace(/\n{3,}/g, "\n\n").trim();

            return {depth: group.depth, content: text};
        })
        .filter((group) => group.content.length > 0);

    if (processedGroups.length === 0) {
        return [];
    }

    const maxDepth = Math.max(...processedGroups.map((g) => g.depth));

    const byDepth = new Map<number, string[]>();
    for (const group of processedGroups) {
        const existing = byDepth.get(group.depth) ?? [];
        existing.push(group.content);
        byDepth.set(group.depth, existing);
    }

    // Deepest quote = oldest user message, alternate roles outward
    const messages: Message[] = [];
    for (let depth = maxDepth; depth >= 0; depth--) {
        const contents = byDepth.get(depth);
        if (!contents || contents.length === 0) continue;

        const combinedContent = contents.join("\n\n");
        const stepsFromMax = maxDepth - depth;
        const role: "user" | "assistant" = stepsFromMax % 2 === 0 ? "user" : "assistant";

        messages.push({role, content: combinedContent});
    }

    return messages;
}

export default {
    async email(
        message: ForwardableEmailMessage,
        environment: Environment,
        _context: ExecutionContext
    ): Promise<void> {
        const from = extractEmailAddress(message.from);

        if (from !== ALLOWED_SENDER) {
            message.setReject("Unauthorized sender");
            return;
        }

        const authResult = verifyEmailAuthentication(message);
        if (!authResult.spfPass || !authResult.dkimPass) {
            message.setReject("Email authentication failed");
            return;
        }

        try {
            const rawEmail = await new Response(message.raw).arrayBuffer();
            const parsed = await PostalMime.parse(rawEmail);

            const subject = parsed.subject ?? "(no subject)";
            const body = parsed.text ?? parsed.html ?? "";

            if (body.length > MAX_BODY_LENGTH) {
                await sendReply(message, environment, subject, "Email body too large");
                return;
            }

            const messages = parseQuotedText(body);
            if (messages.length === 0) {
                await sendReply(message, environment, subject, "(empty body)");
                return;
            }

            const aiOutput = await environment.AI.run("@cf/meta/llama-3.2-3b-instruct", {
                messages,
                max_tokens: 1024,
            });

            await sendReply(message, environment, subject, aiOutput.response ?? "Please try again later.", body);
        } catch (error) {
            console.error(
                "Failed to process email:",
                error instanceof Error ? error.message : "Unknown error"
            );
            throw error;
        }
    },
};

async function sendReply(
    originalMessage: ForwardableEmailMessage,
    environment: Environment,
    originalSubject: string,
    replyBody: string,
    originalBody?: string
): Promise<void> {
    const replyFrom = originalMessage.to;
    const replyTo = originalMessage.from;

    const replySubject = originalSubject.toLowerCase().startsWith("re:")
        ? originalSubject
        : `Re: ${originalSubject}`;

    const fullBody = originalBody
        ? `${replyBody}\n\n${formatQuotedText(originalBody)}`
        : replyBody;

    const mimeMessage = createMimeMessage();
    mimeMessage.setSender({addr: replyFrom});
    mimeMessage.setRecipient(replyTo);
    mimeMessage.setSubject(replySubject);
    mimeMessage.addMessage({
        contentType: "text/plain",
        data: fullBody,
    });

    const messageId = originalMessage.headers.get("Message-ID");
    if (messageId) {
        mimeMessage.setHeader("In-Reply-To", messageId);
        mimeMessage.setHeader("References", messageId);
    }

    const rawMessage = new EmailMessage(replyFrom, replyTo, mimeMessage.asRaw());

    await environment.SEND_EMAIL.send(rawMessage);
}
