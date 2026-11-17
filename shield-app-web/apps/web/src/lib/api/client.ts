import { createShieldClient } from "@shield/api-client";

/** Same-origin: the dev server and production web server both route `/v1` to the Go API. */
export const api = createShieldClient();
