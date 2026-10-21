import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, expect, it, vi } from "vitest";

import { InstallOptionsModal } from "@/components/install-options-modal";
import type { AppStatus } from "@/lib/api";

const branchVersions = ["4.4", "4.5", "4.6", "5.0", "5.1", "5.2", "5.3"];

const apps: AppStatus[] = [
  {
    id: "qbittorrent",
    name: "qBittorrent",
    description: "",
    category: "download",
    installed: false,
    dependencies_satisfied: true,
    install_options: [
      {
        key: "qbittorrent_version",
        label: "qBittorrent version",
        type: "select",
        choices: [
          { value: "4.3", label: "4.3" },
          { value: "4.6", label: "4.6" },
          { value: "5.2", label: "5.2" },
        ],
      },
      {
        key: "libtorrent_branch",
        label: "libtorrent version",
        type: "select",
        choices: [
          { value: "RC_1_2", label: "libtorrent 1.2" },
          { value: "RC_2_0", label: "libtorrent 2.0" },
          { value: "RC_2_1", label: "libtorrent 2.1" },
        ],
        when: { key: "qbittorrent_version", one_of: branchVersions },
      },
    ],
  },
];

const rtorrentApps: AppStatus[] = [
  {
    id: "rtorrent",
    name: "rTorrent",
    description: "",
    category: "download",
    installed: false,
    dependencies_satisfied: true,
    install_options: [
      {
        key: "rtorrent_version",
        label: "rTorrent version",
        type: "select",
        choices: [
          { value: "0.16.x", label: "0.16.x" },
          { value: "0.9.8", label: "0.9.8" },
          { value: "0.9.6", label: "0.9.6" },
        ],
      },
    ],
  },
];

function renderModal(onConfirm = vi.fn()) {
  render(
    <InstallOptionsModal
      appIds={["qbittorrent"]}
      apps={apps}
      onClose={() => {}}
      onConfirm={onConfirm}
    />,
  );
  return onConfirm;
}

function renderRtorrentModal(onConfirm = vi.fn()) {
  render(
    <InstallOptionsModal
      appIds={["rtorrent"]}
      apps={rtorrentApps}
      onClose={() => {}}
      onConfirm={onConfirm}
    />,
  );
  return onConfirm;
}

describe("InstallOptionsModal", () => {
  it("skips the libtorrent branch step for the 4.3 line", async () => {
    const user = userEvent.setup();
    const onConfirm = renderModal();

    // The title is derived from the option label; pin qBittorrent's copy so the
    // shared derivation can't silently regress this consumer.
    expect(screen.getByRole("dialog")).toHaveTextContent("Choose qBittorrent version");

    await user.click(screen.getByRole("radio", { name: "4.3" }));
    await user.click(screen.getByRole("button", { name: "Continue" }));

    expect(screen.queryByRole("radio", { name: "libtorrent 2.0" })).toBeNull();

    await user.click(screen.getByRole("button", { name: "Start install" }));
    expect(onConfirm).toHaveBeenCalledWith({ qbittorrent_version: "4.3" });
  });

  it("shows the libtorrent branch step for libtorrent-2 capable versions", async () => {
    const user = userEvent.setup();
    const onConfirm = renderModal();

    await user.click(screen.getByRole("radio", { name: "4.6" }));
    await user.click(screen.getByRole("button", { name: "Continue" }));

    expect(screen.getByRole("radio", { name: "libtorrent 1.2" })).toBeChecked();
    await user.click(screen.getByRole("radio", { name: "libtorrent 2.0" }));
    await user.click(screen.getByRole("button", { name: "Start install" }));

    expect(onConfirm).toHaveBeenCalledWith({
      qbittorrent_version: "4.6",
      libtorrent_branch: "RC_2_0",
    });
  });

  it("omits libtorrent_patch when no file is supplied", async () => {
    const user = userEvent.setup();
    const onConfirm = renderModal();

    await user.click(screen.getByRole("radio", { name: "5.2" }));
    await user.click(screen.getByRole("button", { name: "Continue" }));
    await user.click(screen.getByRole("button", { name: "Start install" }));

    const submitted = onConfirm.mock.calls[0][0] as Record<string, string>;
    expect(submitted).not.toHaveProperty("libtorrent_patch");
    expect(submitted.qbittorrent_version).toBe("5.2");
  });

  it("attaches a patch and clears it via the in-field clear button", async () => {
    const user = userEvent.setup();
    const onConfirm = renderModal();

    await user.click(screen.getByRole("radio", { name: "5.2" }));
    await user.click(screen.getByRole("button", { name: "Continue" }));

    // No clear affordance until a patch is selected.
    expect(screen.queryByRole("button", { name: "Clear selected patch" })).toBeNull();

    const file = new File(["diff --git a b"], "perf.patch", { type: "text/plain" });
    await user.upload(screen.getByLabelText("Custom libtorrent patch (optional)"), file);

    // The read-only field surfaces the filename and the clear button appears.
    expect(await screen.findByDisplayValue("perf.patch")).toBeInTheDocument();
    await user.click(screen.getByRole("button", { name: "Clear selected patch" }));

    // Cleared: the field empties, the button disappears, and no patch is submitted.
    expect(screen.getByLabelText("Selected patch file")).toHaveValue("");
    expect(screen.queryByRole("button", { name: "Clear selected patch" })).toBeNull();

    await user.click(screen.getByRole("button", { name: "Start install" }));
    const submitted = onConfirm.mock.calls[0][0] as Record<string, string>;
    expect(submitted).not.toHaveProperty("libtorrent_patch");
    expect(submitted.qbittorrent_version).toBe("5.2");
  });

  it("renders a single step keyed to the app's own version option for rTorrent", async () => {
    const user = userEvent.setup();
    const onConfirm = renderRtorrentModal();

    // The modal must not mislabel itself as qBittorrent.
    expect(screen.getByRole("dialog")).toHaveTextContent("Choose rTorrent version");
    expect(screen.getByRole("dialog")).not.toHaveTextContent("qBittorrent");

    // Version choices are present and the first is selected by default.
    expect(screen.getByRole("radio", { name: "0.16.x" })).toBeChecked();
    // Single step: it commits directly instead of advancing to a second step.
    expect(screen.queryByRole("button", { name: "Continue" })).toBeNull();

    await user.click(screen.getByRole("radio", { name: "0.9.6" }));
    await user.click(screen.getByRole("button", { name: "Start install" }));

    expect(onConfirm).toHaveBeenCalledWith({ rtorrent_version: "0.9.6" });
  });
});
