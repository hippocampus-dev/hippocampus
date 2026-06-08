import { EmailMessage } from "cloudflare:email";
import { Agent, type AgentEmail } from "agents";
import { createMimeMessage } from "mimetext";
import PostalMime from "postal-mime";
import { SlackClient } from "./slack";

export interface Environment extends Cloudflare.Env {
  SLACK_BOT_TOKEN: string;
  SLACK_SIGNING_SECRET: string;
  SLACK_CHANNEL_ID: string;
  SENDER_ALLOWLIST: string;
}

export const AGENT_ADDRESS = "agent@kaidotio.dev";
export const MAX_BODY_LENGTH = 100000;
const MAX_OUTPUT_TOKENS = 1024;
const DRAFT_TTL_SECONDS = 24 * 60 * 60;

export interface DraftRow {
  draft_id: string;
  in_reply_to: string;
  references_chain: string;
  from_addr: string;
  subject: string;
  draft_reply: string;
  nonce: string;
  status: "pending" | "approved" | "denied" | "expired";
  slack_channel: string;
  slack_ts: string;
  created_at: number;
  resolved_at: number | null;
}

export type ResolveResult =
  | { ok: true; outcome: "approved" | "denied" }
  | {
      ok: false;
      reason:
        | "not-found"
        | "already-resolved"
        | "invalid-nonce"
        | "send-failed";
      detail?: string;
    };

export type EditPrepareResult =
  | {
      ok: true;
      subject: string;
      from: string;
      draftReply: string;
    }
  | {
      ok: false;
      reason: "not-found" | "already-resolved" | "invalid-nonce";
    };

export type EditApplyResult =
  | {
      ok: true;
      draftId: string;
      nonce: string;
      from: string;
      subject: string;
      draftReply: string;
      slackChannel: string;
      slackTs: string;
    }
  | {
      ok: false;
      reason: "not-found" | "already-resolved" | "invalid-nonce";
    };

export class EmailAgent extends Agent<Environment> {
  async onStart(): Promise<void> {
    this.sql`
      CREATE TABLE IF NOT EXISTS drafts (
        draft_id TEXT PRIMARY KEY,
        in_reply_to TEXT NOT NULL,
        references_chain TEXT NOT NULL,
        from_addr TEXT NOT NULL,
        subject TEXT NOT NULL,
        draft_reply TEXT NOT NULL,
        nonce TEXT NOT NULL,
        status TEXT NOT NULL,
        slack_channel TEXT NOT NULL DEFAULT '',
        slack_ts TEXT NOT NULL DEFAULT '',
        created_at INTEGER NOT NULL,
        resolved_at INTEGER
      )
    `;
  }

  async onEmail(email: AgentEmail): Promise<void> {
    const rawBytes = await email.getRaw();
    const parsed = await PostalMime.parse(rawBytes);
    const from = (parsed.from?.address ?? email.from).toLowerCase().trim();

    if (from === AGENT_ADDRESS) {
      email.setReject("Self-loop rejected");
      return;
    }

    const allowlist = new Set(
      this.env.SENDER_ALLOWLIST.split(",")
        .map((entry) => entry.trim().toLowerCase())
        .filter((entry) => entry.length > 0),
    );
    if (!allowlist.has(from)) {
      email.setReject("Sender not allowlisted");
      return;
    }

    const autoSubmitted = email.headers.get("Auto-Submitted");
    if (autoSubmitted !== null && autoSubmitted.toLowerCase() !== "no") {
      email.setReject("Auto-submitted email rejected");
      return;
    }
    const precedence = email.headers.get("Precedence");
    if (precedence !== null && /^(list|bulk|junk)$/i.test(precedence)) {
      email.setReject("Bulk/list email rejected");
      return;
    }

    const body = (parsed.text ?? parsed.html ?? "").slice(0, MAX_BODY_LENGTH);
    const subject = parsed.subject ?? "(no subject)";
    const inReplyTo = sanitizeMessageId(email.headers.get("Message-ID"));
    const incomingReferences = parseReferences(email.headers.get("References"));
    const referencesChain = [...incomingReferences, inReplyTo]
      .filter((entry) => entry.length > 0)
      .join(" ");

    if (inReplyTo.length > 0) {
      const existing = this.sql<{ draft_id: string }>`
        SELECT draft_id FROM drafts WHERE in_reply_to = ${inReplyTo}
      `;
      if (existing.length > 0) {
        return;
      }
    }

    const aiOutput = (await this.env.AI.run("@cf/meta/llama-3.2-3b-instruct", {
      messages: [
        {
          role: "system",
          content:
            "You are an email assistant. Draft a concise, polite reply based on the incoming message.",
        },
        {
          role: "user",
          content: `From: ${from}\nSubject: ${subject}\n\n${body}`,
        },
      ],
      max_tokens: MAX_OUTPUT_TOKENS,
    })) as AiTextGenerationOutput;
    const draftReply = aiOutput.response ?? "(empty draft)";
    const truncated = aiOutput.usage?.completion_tokens === MAX_OUTPUT_TOKENS;

    const draftId = crypto.randomUUID();
    const nonce = crypto.randomUUID();
    const now = Math.floor(Date.now() / 1000);
    this.sql`
      INSERT INTO drafts (
        draft_id, in_reply_to, references_chain, from_addr, subject,
        draft_reply, nonce, status, created_at
      ) VALUES (
        ${draftId}, ${inReplyTo}, ${referencesChain}, ${from}, ${subject},
        ${draftReply}, ${nonce}, 'pending', ${now}
      )
    `;

    const slack = new SlackClient(
      this.env.SLACK_BOT_TOKEN,
      this.env.SLACK_SIGNING_SECRET,
      this.env.SLACK_CHANNEL_ID,
    );
    const { channel, ts } = await slack.postApproval({
      draftId,
      nonce,
      from,
      subject,
      draftReply,
      truncated,
    });
    this.sql`
      UPDATE drafts
      SET slack_channel = ${channel}, slack_ts = ${ts}
      WHERE draft_id = ${draftId}
    `;

    await this.schedule(DRAFT_TTL_SECONDS, "expireDraft", { draftId });
  }

  async expireDraft(payload: { draftId: string }): Promise<void> {
    const now = Math.floor(Date.now() / 1000);
    this.sql`
      UPDATE drafts
      SET status = 'expired', resolved_at = ${now}
      WHERE draft_id = ${payload.draftId} AND status = 'pending'
    `;
  }

  async resolveDraft(
    draftId: string,
    action: "approve" | "deny",
    providedNonce: string,
  ): Promise<ResolveResult> {
    const rows = this.sql<DraftRow>`
      SELECT * FROM drafts WHERE draft_id = ${draftId}
    `;
    const draft = rows[0];
    if (!draft) {
      return { ok: false, reason: "not-found" };
    }
    if (draft.status !== "pending") {
      return { ok: false, reason: "already-resolved" };
    }
    if (draft.nonce !== providedNonce) {
      return { ok: false, reason: "invalid-nonce" };
    }

    if (action === "approve") {
      try {
        await sendReply(this.env, draft);
      } catch (error) {
        return {
          ok: false,
          reason: "send-failed",
          detail: error instanceof Error ? error.message : String(error),
        };
      }
    }

    const now = Math.floor(Date.now() / 1000);
    const nextStatus = action === "approve" ? "approved" : "denied";
    this.sql`
      UPDATE drafts
      SET status = ${nextStatus}, resolved_at = ${now}
      WHERE draft_id = ${draftId}
    `;

    return { ok: true, outcome: nextStatus };
  }

  async prepareEdit(
    draftId: string,
    providedNonce: string,
  ): Promise<EditPrepareResult> {
    const rows = this.sql<DraftRow>`
      SELECT * FROM drafts WHERE draft_id = ${draftId}
    `;
    const draft = rows[0];
    if (!draft) {
      return { ok: false, reason: "not-found" };
    }
    if (draft.status !== "pending") {
      return { ok: false, reason: "already-resolved" };
    }
    if (draft.nonce !== providedNonce) {
      return { ok: false, reason: "invalid-nonce" };
    }
    return {
      ok: true,
      subject: draft.subject,
      from: draft.from_addr,
      draftReply: draft.draft_reply,
    };
  }

  async applyEdit(
    draftId: string,
    providedNonce: string,
    newReply: string,
  ): Promise<EditApplyResult> {
    const rows = this.sql<DraftRow>`
      SELECT * FROM drafts WHERE draft_id = ${draftId}
    `;
    const draft = rows[0];
    if (!draft) {
      return { ok: false, reason: "not-found" };
    }
    if (draft.status !== "pending") {
      return { ok: false, reason: "already-resolved" };
    }
    if (draft.nonce !== providedNonce) {
      return { ok: false, reason: "invalid-nonce" };
    }

    const newNonce = crypto.randomUUID();
    this.sql`
      UPDATE drafts
      SET draft_reply = ${newReply}, nonce = ${newNonce}
      WHERE draft_id = ${draftId}
    `;

    return {
      ok: true,
      draftId,
      nonce: newNonce,
      from: draft.from_addr,
      subject: draft.subject,
      draftReply: newReply,
      slackChannel: draft.slack_channel,
      slackTs: draft.slack_ts,
    };
  }
}

function sanitizeMessageId(raw: string | null): string {
  if (raw === null) {
    return "";
  }
  const cleaned = raw.replace(/[\r\n]/g, "").trim();
  if (cleaned.length === 0) {
    return "";
  }
  if (cleaned.startsWith("<") && cleaned.endsWith(">")) {
    return cleaned;
  }
  return `<${cleaned.replace(/^<|>$/g, "")}>`;
}

function parseReferences(raw: string | null): string[] {
  if (raw === null) {
    return [];
  }
  const matches = raw.replace(/[\r\n]/g, " ").match(/<[^>]+>/g);
  return matches ?? [];
}

async function sendReply(
  environment: Environment,
  draft: DraftRow,
): Promise<void> {
  const replySubject = draft.subject.toLowerCase().startsWith("re:")
    ? draft.subject
    : `Re: ${draft.subject}`;

  const mime = createMimeMessage();
  mime.setSender({ addr: AGENT_ADDRESS });
  mime.setRecipient(draft.from_addr);
  mime.setSubject(replySubject);
  if (draft.in_reply_to.length > 0) {
    mime.setHeader("In-Reply-To", draft.in_reply_to);
  }
  if (draft.references_chain.length > 0) {
    mime.setHeader("References", draft.references_chain);
  }
  mime.addMessage({ contentType: "text/plain", data: draft.draft_reply });

  const outbound = new EmailMessage(
    AGENT_ADDRESS,
    draft.from_addr,
    mime.asRaw(),
  );
  await environment.EMAIL.send(outbound);
}
