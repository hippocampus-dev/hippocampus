import { createAddressBasedEmailResolver, routeAgentEmail } from "agents";
import { EmailAgent, type Environment } from "./agent";
import { handleSlackInteractive } from "./slack";

export { EmailAgent };

export default {
  async email(
    message: ForwardableEmailMessage,
    environment: Environment,
    _context: ExecutionContext,
  ): Promise<void> {
    await routeAgentEmail(message, environment, {
      resolver: createAddressBasedEmailResolver("EmailAgent"),
    });
  },

  async fetch(
    request: Request,
    environment: Environment,
    context: ExecutionContext,
  ): Promise<Response> {
    const url = new URL(request.url);
    if (url.pathname !== "/slack/interactive" || request.method !== "POST") {
      return new Response("Not found", { status: 404 });
    }
    return await handleSlackInteractive(request, environment, context);
  },
} satisfies ExportedHandler<Environment>;
