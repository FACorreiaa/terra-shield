import { createFileRoute, redirect } from "@tanstack/react-router";

/** A shared country link (`/c/PT`): the country's overview on the live map. */
export const Route = createFileRoute("/c/$iso2")({
  beforeLoad: ({ params, location }) => {
    // An event in the link stays open beside the country.
    const { event } = location.search as { event?: unknown };
    // oxlint-disable-next-line typescript/only-throw-error -- the router's redirect is thrown by design
    throw redirect({
      to: "/countries/$iso2",
      params: { iso2: params.iso2 },
      search: typeof event === "string" ? { event } : {},
      replace: true,
    });
  },
});
