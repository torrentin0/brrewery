import type { ReactNode } from "react";
import { useMemo } from "react";
import { useQuery } from "@tanstack/react-query";
import { IconArrowDown, IconArrowUp, IconArrowsUpDown } from "@tabler/icons-react";

import { ChartPanelControls } from "@/components/chart-panel-controls";
import { VnstatRangeSelect } from "@/components/vnstat-range-select";
import { Card } from "@/components/ui/card";
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from "@/components/ui/table";
import { useSetting } from "@/hooks/use-setting";
import { getVnstatReport, type TrafficPeriod } from "@/lib/api";
import { formatBytes } from "@/lib/format";
import {
  DEFAULT_VNSTAT_RANGE,
  isVnstatRangeId,
  VNSTAT_RANGE_OPTIONS,
  vnstatReportRequest,
  type VnstatRangeId,
} from "@/lib/vnstat-range";

export function VnstatPanel() {
  const vnstat = useQuery({
    queryKey: ["vnstat"],
    queryFn: () => getVnstatReport(vnstatReportRequest()),
    refetchInterval: 60_000,
  });
  const [range, setRange] = useSetting<VnstatRangeId>(
    "vnstat-range",
    DEFAULT_VNSTAT_RANGE,
    isVnstatRangeId,
  );
  const report = vnstat.data;
  const summary = useMemo(() => {
    const days = report?.days ?? [];
    const months = report?.months ?? [];
    const today = days.length > 0 ? days[days.length - 1] : null;
    const thisMonth = months.length > 0 ? months[months.length - 1] : null;
    const allTime = report?.total ?? null;
    return { today, thisMonth, allTime };
  }, [report]);

  const tableConfig = useMemo(() => {
    const option = VNSTAT_RANGE_OPTIONS.find((o) => o.id === range) ?? VNSTAT_RANGE_OPTIONS[0];
    const periods = report?.[option.source] ?? [];
    if (option.sort === "total") {
      return {
        periods: [...periods]
          .sort((a, b) => b.rx_bytes + b.tx_bytes - (a.rx_bytes + a.tx_bytes))
          .slice(0, option.limit),
        reverse: false,
      };
    }
    return { periods: periods.slice(-option.limit), reverse: true };
  }, [range, report]);

  if (vnstat.isLoading) {
    return (
      <Panel title="vnStat - Historic traffic">
        <p className="text-sm text-muted-foreground">Loading vnstat data…</p>
      </Panel>
    );
  }

  if (vnstat.isError) {
    return (
      <Panel title="vnStat - Historic traffic">
        <p className="text-sm text-destructive">{vnstat.error.message}</p>
      </Panel>
    );
  }
  if (!report?.installed) {
    return (
      <Panel title="vnStat - Historic traffic">
        <p className="text-sm text-muted-foreground">
          {report?.message ?? "vnstat is not installed on this system."}
        </p>
        <p className="mt-2 text-xs text-muted-foreground">
          Install vnstat and ensure the daemon is collecting data for your interfaces.
        </p>
      </Panel>
    );
  }

  return (
    <Panel
      title="vnStat - Historic traffic"
      headerRight={
        <ChartPanelControls
          timeRange={<VnstatRangeSelect value={range} onChange={setRange} />}
        />
      }
    >
      <div>
        <TrafficSummaryCards
          today={summary.today}
          thisMonth={summary.thisMonth}
          allTime={summary.allTime}
        />
        <TrafficTable periods={tableConfig.periods} reverse={tableConfig.reverse} />
      </div>
    </Panel>
  );
}

function TrafficSummaryCards({
  today,
  thisMonth,
  allTime,
}: {
  today: TrafficPeriod | null;
  thisMonth: TrafficPeriod | null;
  allTime: TrafficPeriod | null;
}) {
  const cards = [
    { label: "Today", period: today },
    { label: "This Month", period: thisMonth },
    { label: "All Time", period: allTime },
  ];

  return (
    <div className="mb-4 grid grid-cols-1 gap-3 sm:grid-cols-3">
      {cards.map(({ label, period }) => {
        const rx = period?.rx_bytes ?? 0;
        const tx = period?.tx_bytes ?? 0;
        const total = rx + tx;

        return (
          <div
            key={label}
            className="flex flex-col items-center justify-center rounded-lg border border-border bg-card/60 p-3 text-center"
          >
            <span className="text-xs font-medium text-muted-foreground">{label}</span>
            <div className="my-1">
              <span className="text-xl font-bold tabular-nums text-foreground">
                {period ? formatBytes(total) : "—"}
              </span>
            </div>
            <div className="flex items-center gap-3 text-xs tabular-nums">
              <span className="inline-flex items-center gap-1 text-sky-400">
                <IconArrowDown className="size-3" aria-hidden="true" />
                {period ? formatBytes(rx) : "—"}
              </span>
              <span className="inline-flex items-center gap-1 text-emerald-400">
                <IconArrowUp className="size-3" aria-hidden="true" />
                {period ? formatBytes(tx) : "—"}
              </span>
            </div>
          </div>
        );
      })}
    </div>
  );
}

function Panel({
  title,
  subtitle,
  headerRight,
  children,
}: {
  title: string;
  subtitle?: string;
  headerRight?: ReactNode;
  children: ReactNode;
}) {
  return (
    <Card className="gap-0 p-4">
      <div className="mb-4 flex items-start justify-between gap-3">
        <div>
          <h2 className="text-sm font-semibold text-foreground">{title}</h2>
          {subtitle && <p className="text-xs text-muted-foreground">{subtitle}</p>}
        </div>
        {headerRight}
      </div>
      {children}
    </Card>
  );
}

function TrafficTable({ periods, reverse = true }: { periods: TrafficPeriod[]; reverse?: boolean }) {
  if (periods.length === 0) {
    return (
      <div>
        <p className="text-sm text-muted-foreground">No data recorded yet.</p>
      </div>
    );
  }

  const rows = reverse ? [...periods].reverse() : periods;

  return (
    <div>
      <div className="overflow-x-auto rounded-md border border-border">
        <Table className="min-w-full text-left text-sm">
          <TableHeader className="bg-muted text-muted-foreground">
            <TableRow>
              <TableHead className="px-3 py-2 font-medium text-foreground">Period</TableHead>
              <TableHead className="px-3 py-2 font-medium text-sky-400">
                <span className="inline-flex items-center gap-1">
                  <IconArrowDown className="size-3.5" aria-hidden="true" />
                  Download
                </span>
              </TableHead>
              <TableHead className="px-3 py-2 font-medium text-emerald-400">
                <span className="inline-flex items-center gap-1">
                  <IconArrowUp className="size-3.5" aria-hidden="true" />
                  Upload
                </span>
              </TableHead>
              <TableHead className="px-3 py-2 font-medium text-orange-400">
                <span className="inline-flex items-center gap-1">
                  <IconArrowsUpDown className="size-3.5" aria-hidden="true" />
                  Total
                </span>
              </TableHead>
            </TableRow>
          </TableHeader>
          <TableBody>
            {rows.map((row) => (
              <TableRow key={row.label} className="border-t border-border">
                <TableCell className="px-3 py-2 tabular-nums text-foreground">{row.label}</TableCell>
                <TableCell className="px-3 py-2 tabular-nums text-sky-400">{formatBytes(row.rx_bytes)}</TableCell>
                <TableCell className="px-3 py-2 tabular-nums text-emerald-400">{formatBytes(row.tx_bytes)}</TableCell>
                <TableCell className="px-3 py-2 tabular-nums text-orange-400">{formatBytes(row.rx_bytes + row.tx_bytes)}</TableCell>
              </TableRow>
            ))}
          </TableBody>
        </Table>
      </div>
    </div>
  );
}
