import {
  type EditApplyResult,
  type EditPrepareResult,
  type Environment,
  MAX_BODY_LENGTH,
  type ResolveResult,
} from "./agent";

const SLACK_TIMESTAMP_TOLERANCE_SECONDS = 5 * 60;
const MODAL_CALLBACK_ID = "edit_draft";
const MODAL_INPUT_BLOCK_ID = "edit_body";
const MODAL_INPUT_ACTION_ID = "edit_body_input";

type SlackTextObject =
  | { type: "plain_text"; text: string; emoji?: boolean }
  | { type: "mrkdwn"; text: string; verbatim?: boolean };

interface SlackButton {
  type: "button";
  action_id: string;
  text: { type: "plain_text"; text: string };
  style?: "primary" | "danger";
}

interface SlackPlainTextInput {
  type: "plain_text_input";
  action_id: string;
  multiline?: boolean;
  initial_value?: string;
}

type SlackBlock =
  | { type: "section"; text: SlackTextObject; block_id?: string }
  | { type: "actions"; block_id?: string; elements: SlackButton[] }
  | {
      type: "input";
      block_id: string;
      label: SlackTextObject;
      element: SlackPlainTextInput;
    };

interface SlackPostMessageResponse {
  ok: boolean;
  channel?: string;
  ts?: string;
  error?: string;
}

type BlockActionsPayload = {
  type: "block_actions";
  trigger_id?: string;
  response_url?: string;
  actions?: { action_id: string; block_id: string }[];
};

type ViewSubmissionPayload = {
  type: "view_submission";
  view?: {
    callback_id: string;
    private_metadata: string;
    state: {
      values: Record<string, Record<string, { value?: string }>>;
    };
  };
};

type SlackPayload = BlockActionsPayload | ViewSubmissionPayload;

interface ApprovalParams {
  draftId: string;
  nonce: string;
  from: string;
  subject: string;
  draftReply: string;
  truncated?: boolean;
}

export class SlackClient {
  constructor(
    private readonly botToken: string,
    private readonly signingSecret: string,
    private readonly channelId: string,
  ) {}

  async postApproval(
    params: ApprovalParams,
  ): Promise<{ channel: string; ts: string }> {
    const requestBody = {
      channel: this.channelId,
      text: `Draft for ${params.from}: ${params.subject}`,
      blocks: buildApprovalBlocks(params),
    };
    const response = await fetch("https://slack.com/api/chat.postMessage", {
      method: "POST",
      headers: this.jsonHeaders(),
      body: JSON.stringify(requestBody),
    });
    const data = (await response.json()) as SlackPostMessageResponse;
    if (!data.ok || data.channel === undefined || data.ts === undefined) {
      throw new Error(
        `Slack chat.postMessage failed: ${data.error ?? "unknown"}`,
      );
    }
    return { channel: data.channel, ts: data.ts };
  }

  async updateMessage(params: {
    channel: string;
    ts: string;
    draftId: string;
    nonce: string;
    from: string;
    subject: string;
    draftReply: string;
  }): Promise<void> {
    const requestBody = {
      channel: params.channel,
      ts: params.ts,
      text: `Draft for ${params.from}: ${params.subject}`,
      blocks: buildApprovalBlocks(params),
    };
    const response = await fetch("https://slack.com/api/chat.update", {
      method: "POST",
      headers: this.jsonHeaders(),
      body: JSON.stringify(requestBody),
    });
    const data = (await response.json()) as { ok: boolean; error?: string };
    if (!data.ok) {
      throw new Error(`Slack chat.update failed: ${data.error ?? "unknown"}`);
    }
  }

  async openEditModal(params: {
    triggerId: string;
    draftId: string;
    nonce: string;
    subject: string;
    draftReply: string;
  }): Promise<void> {
    const blocks: SlackBlock[] = [
      {
        type: "section",
        text: { type: "mrkdwn", text: `*Subject:* ${params.subject}` },
      },
      {
        type: "input",
        block_id: MODAL_INPUT_BLOCK_ID,
        label: { type: "plain_text", text: "Reply body" },
        element: {
          type: "plain_text_input",
          action_id: MODAL_INPUT_ACTION_ID,
          multiline: true,
          initial_value: params.draftReply,
        },
      },
    ];
    const view = {
      type: "modal",
      callback_id: MODAL_CALLBACK_ID,
      private_metadata: JSON.stringify({
        draftId: params.draftId,
        nonce: params.nonce,
      }),
      title: { type: "plain_text", text: "Edit Draft" },
      submit: { type: "plain_text", text: "Save" },
      close: { type: "plain_text", text: "Cancel" },
      blocks,
    };
    const response = await fetch("https://slack.com/api/views.open", {
      method: "POST",
      headers: this.jsonHeaders(),
      body: JSON.stringify({ trigger_id: params.triggerId, view }),
    });
    const data = (await response.json()) as { ok: boolean; error?: string };
    if (!data.ok) {
      throw new Error(`Slack views.open failed: ${data.error ?? "unknown"}`);
    }
  }

  async postResponseUrl(
    responseUrl: string,
    result: ResolveResult | { ok: false; reason: string; detail?: string },
  ): Promise<void> {
    const text = result.ok
      ? `:white_check_mark: ${result.outcome}`
      : `:x: failed (${result.reason}${result.detail !== undefined ? `: ${result.detail}` : ""})`;
    const body = { replace_original: false, text };
    await fetch(responseUrl, {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify(body),
    });
  }

  async verifySignature(request: Request, body: string): Promise<boolean> {
    const timestamp = request.headers.get("X-Slack-Request-Timestamp");
    const signature = request.headers.get("X-Slack-Signature");
    if (timestamp === null || signature === null) {
      return false;
    }
    const ts = Number.parseInt(timestamp, 10);
    if (Number.isNaN(ts)) {
      return false;
    }
    const now = Math.floor(Date.now() / 1000);
    if (Math.abs(now - ts) > SLACK_TIMESTAMP_TOLERANCE_SECONDS) {
      return false;
    }

    const basestring = `v0:${timestamp}:${body}`;
    const key = await crypto.subtle.importKey(
      "raw",
      new TextEncoder().encode(this.signingSecret),
      { name: "HMAC", hash: "SHA-256" },
      false,
      ["sign"],
    );
    const sigBytes = await crypto.subtle.sign(
      "HMAC",
      key,
      new TextEncoder().encode(basestring),
    );
    const expectedHex = Array.from(new Uint8Array(sigBytes))
      .map((byte) => byte.toString(16).padStart(2, "0"))
      .join("");
    const expected = `v0=${expectedHex}`;

    const signatureBytes = new TextEncoder().encode(signature);
    const expectedBytes = new TextEncoder().encode(expected);
    const length = Math.max(signatureBytes.length, expectedBytes.length);
    let mismatch = signatureBytes.length ^ expectedBytes.length;
    for (let i = 0; i < length; i++) {
      const a = signatureBytes[i] ?? 0;
      const b = expectedBytes[i] ?? 0;
      mismatch |= a ^ b;
    }
    return mismatch === 0;
  }

  private jsonHeaders(): Record<string, string> {
    return {
      "Content-Type": "application/json",
      Authorization: `Bearer ${this.botToken}`,
    };
  }
}

function buildApprovalBlocks(params: ApprovalParams): SlackBlock[] {
  const { draftId, nonce, from, subject, draftReply, truncated } = params;
  const blocks: SlackBlock[] = [
    {
      type: "section",
      text: {
        type: "mrkdwn",
        text: `*From:* ${from}\n*Subject:* ${subject}`,
      },
    },
    {
      type: "section",
      text: { type: "mrkdwn", text: `\`\`\`${draftReply}\`\`\`` },
    },
  ];
  if (truncated === true) {
    blocks.push({
      type: "section",
      text: {
        type: "mrkdwn",
        text: ":warning: Draft was truncated at the token limit — consider editing before sending.",
      },
    });
  }
  blocks.push({
    type: "actions",
    block_id: draftId,
    elements: [
      {
        type: "button",
        action_id: `approve:${nonce}`,
        text: { type: "plain_text", text: "Approve" },
        style: "primary",
      },
      {
        type: "button",
        action_id: `edit:${nonce}`,
        text: { type: "plain_text", text: "Edit" },
      },
      {
        type: "button",
        action_id: `deny:${nonce}`,
        text: { type: "plain_text", text: "Deny" },
        style: "danger",
      },
    ],
  });
  return blocks;
}

export async function handleSlackInteractive(
  request: Request,
  environment: Environment,
  context: ExecutionContext,
): Promise<Response> {
  const slack = new SlackClient(
    environment.SLACK_BOT_TOKEN,
    environment.SLACK_SIGNING_SECRET,
    environment.SLACK_CHANNEL_ID,
  );

  const body = await request.text();
  const valid = await slack.verifySignature(request, body);
  if (!valid) {
    return new Response("Invalid signature", { status: 401 });
  }

  const params = new URLSearchParams(body);
  const payloadString = params.get("payload");
  if (payloadString === null) {
    return new Response("Missing payload", { status: 400 });
  }

  let payload: SlackPayload;
  try {
    payload = JSON.parse(payloadString);
  } catch {
    return new Response("Invalid payload", { status: 400 });
  }

  const id = environment.EmailAgent.idFromName("singleton");
  const stub = environment.EmailAgent.get(id);

  if (payload.type === "block_actions") {
    return await handleBlockActions(payload, slack, stub, context);
  }
  if (payload.type === "view_submission") {
    return await handleViewSubmission(payload, slack, stub, context);
  }
  return new Response("Unknown payload type", { status: 400 });
}

async function handleBlockActions(
  payload: BlockActionsPayload,
  slack: SlackClient,
  stub: DurableObjectStub<import("./agent").EmailAgent>,
  context: ExecutionContext,
): Promise<Response> {
  const action = payload.actions?.[0];
  const responseUrl = payload.response_url;
  const triggerId = payload.trigger_id;
  if (action === undefined || responseUrl === undefined) {
    return new Response("Incomplete payload", { status: 400 });
  }
  const draftId = action.block_id;
  const [verb, nonce] = action.action_id.split(":");

  if (verb === "approve" || verb === "deny") {
    context.waitUntil(
      (async () => {
        const result = await stub.resolveDraft(draftId, verb, nonce);
        await slack.postResponseUrl(responseUrl, result);
      })(),
    );
    return new Response(null, { status: 200 });
  }

  if (verb === "edit") {
    if (triggerId === undefined) {
      return new Response("Missing trigger_id", { status: 400 });
    }
    const prep: EditPrepareResult = await stub.prepareEdit(draftId, nonce);
    if (!prep.ok) {
      await slack.postResponseUrl(responseUrl, {
        ok: false,
        reason: prep.reason,
      });
      return new Response(null, { status: 200 });
    }
    try {
      await slack.openEditModal({
        triggerId,
        draftId,
        nonce,
        subject: prep.subject,
        draftReply: prep.draftReply,
      });
    } catch (error) {
      await slack.postResponseUrl(responseUrl, {
        ok: false,
        reason: "send-failed",
        detail: error instanceof Error ? error.message : String(error),
      });
    }
    return new Response(null, { status: 200 });
  }

  return new Response("Unknown action", { status: 400 });
}

async function handleViewSubmission(
  payload: ViewSubmissionPayload,
  slack: SlackClient,
  stub: DurableObjectStub<import("./agent").EmailAgent>,
  context: ExecutionContext,
): Promise<Response> {
  const view = payload.view;
  if (view === undefined || view.callback_id !== MODAL_CALLBACK_ID) {
    return new Response("Unknown view", { status: 400 });
  }

  let metadata: { draftId?: string; nonce?: string };
  try {
    metadata = JSON.parse(view.private_metadata);
  } catch {
    return new Response("Invalid private_metadata", { status: 400 });
  }
  const draftId = metadata.draftId;
  const nonce = metadata.nonce;
  if (draftId === undefined || nonce === undefined) {
    return new Response("Incomplete private_metadata", { status: 400 });
  }

  const newReply =
    view.state.values[MODAL_INPUT_BLOCK_ID]?.[MODAL_INPUT_ACTION_ID]?.value;
  if (newReply === undefined || newReply.length === 0) {
    return new Response(
      JSON.stringify({
        response_action: "errors",
        errors: { [MODAL_INPUT_BLOCK_ID]: "Reply body cannot be empty" },
      }),
      { status: 200, headers: { "Content-Type": "application/json" } },
    );
  }
  if (newReply.length > MAX_BODY_LENGTH) {
    return new Response(
      JSON.stringify({
        response_action: "errors",
        errors: {
          [MODAL_INPUT_BLOCK_ID]: `Reply body exceeds ${MAX_BODY_LENGTH} characters`,
        },
      }),
      { status: 200, headers: { "Content-Type": "application/json" } },
    );
  }

  context.waitUntil(
    (async () => {
      const result: EditApplyResult = await stub.applyEdit(
        draftId,
        nonce,
        newReply,
      );
      if (!result.ok) {
        return;
      }
      await slack.updateMessage({
        channel: result.slackChannel,
        ts: result.slackTs,
        draftId: result.draftId,
        nonce: result.nonce,
        from: result.from,
        subject: result.subject,
        draftReply: result.draftReply,
      });
    })(),
  );
  return new Response(null, { status: 200 });
}
