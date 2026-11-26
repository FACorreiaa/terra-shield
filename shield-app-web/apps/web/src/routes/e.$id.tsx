import { createFileRoute, redirect } from "@tanstack/react-router";

/** A shared event link (`/e/<id>`): the live map with that event open. */
export const Route = createFileRoute("/e/$id")({
  beforeLoad: ({ params }) => {
    // oxlint-disable-next-line typescript/only-throw-error -- the router's redirect is thrown by design
    throw redirect({ to: "/map", search: { event: params.id }, replace: true });
  },
});
