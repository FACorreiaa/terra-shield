import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import type { ReactElement, ReactNode } from "react";

const queryClient = new QueryClient({ defaultOptions: { queries: { retry: 1 } } });

function AppProviders({ children }: { children: ReactNode }): ReactElement {
  return <QueryClientProvider client={queryClient}>{children}</QueryClientProvider>;
}

export { AppProviders };
