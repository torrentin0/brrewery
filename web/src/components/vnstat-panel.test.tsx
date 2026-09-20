import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, expect, it, vi } from "vitest";

import { VnstatPanel } from "@/components/vnstat-panel";

describe("VnstatPanel", () => {
  it("shows missing message when vnstat is not installed", async () => {
    vi.stubGlobal(
      "fetch",
      vi.fn().mockResolvedValue({
        ok: true,
        status: 200,
        json: async () => ({
          installed: false,
          message: "vnstat is not installed on this system.",
        }),
      }),
    );

    const client = new QueryClient({
      defaultOptions: { queries: { retry: false } },
    });

    render(
      <QueryClientProvider client={client}>
        <VnstatPanel />
      </QueryClientProvider>,
    );

    expect(await screen.findByText(/vnstat is not installed/i)).toBeInTheDocument();
  });

  it("renders traffic tables when installed", async () => {
    vi.stubGlobal(
      "fetch",
      vi.fn().mockResolvedValue({
        ok: true,
        status: 200,
        json: async () => ({
          installed: true,
          version: "2.12",
          months: [{ label: "2026-05", rx_bytes: 1000, tx_bytes: 500 }],
          days: [
            { label: "2026-05-31", rx_bytes: 100, tx_bytes: 50 },
            { label: "2026-05-30", rx_bytes: 300, tx_bytes: 100 },
          ],
          total: { label: "All Time", rx_bytes: 50000, tx_bytes: 25000 },
        }),
      }),
    );

    const client = new QueryClient({
      defaultOptions: { queries: { retry: false } },
    });

    render(
      <QueryClientProvider client={client}>
        <VnstatPanel />
      </QueryClientProvider>,
    );

    expect(await screen.findByText("2026-05-31")).toBeInTheDocument();
    expect(screen.getByText("Today")).toBeInTheDocument();
    expect(screen.getByText("This Month")).toBeInTheDocument();
    expect(screen.getByText("All Time")).toBeInTheDocument();

    const user = userEvent.setup();

    await user.click(screen.getByRole("combobox", { name: "vnStat range" }));
    await user.click(await screen.findByRole("option", { name: "Last 12 months" }));
    expect(screen.getByText("2026-05")).toBeInTheDocument();

    await user.click(screen.getByRole("combobox", { name: "vnStat range" }));
    await user.click(await screen.findByRole("option", { name: "Top 10 days overall" }));
    const top10Rows = screen.getAllByRole("row").slice(1);
    expect(top10Rows[0]).toHaveTextContent("2026-05-30");
    expect(top10Rows[1]).toHaveTextContent("2026-05-31");
  });
});
