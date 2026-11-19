// KD: Zendesk-style workspace tabs. Every ticket, contact or customer an agent
// opens gets a tab in the strip above the page, so they can jump between open
// work without losing their place. Tabs persist per agent in localStorage.
import { userStorage } from "@/composables/userStorage";
import { call } from "frappe-ui";
import { defineStore } from "pinia";
import type { RouteLocationNormalizedLoaded, RouteLocationRaw } from "vue-router";

export type TabKind = "ticket" | "contact" | "customer";

export interface WorkspaceTab {
  kind: TabKind;
  id: string;
  title: string;
}

const MAX_TABS = 12;

const KIND_BY_ROUTE: Record<string, { kind: TabKind; param: string }> = {
  TicketAgent: { kind: "ticket", param: "ticketId" },
  Contact: { kind: "contact", param: "id" },
  Customer: { kind: "customer", param: "id" },
};

export function tabFromRoute(
  route: RouteLocationNormalizedLoaded
): { kind: TabKind; id: string } | null {
  const match = KIND_BY_ROUTE[route.name as string];
  if (!match) return null;
  const id = route.params[match.param];
  return id ? { kind: match.kind, id: String(id) } : null;
}

export function tabRoute(tab: Pick<WorkspaceTab, "kind" | "id">): RouteLocationRaw {
  switch (tab.kind) {
    case "ticket":
      return { name: "TicketAgent", params: { ticketId: tab.id } };
    case "contact":
      return { name: "Contact", params: { id: tab.id } };
    case "customer":
      return { name: "Customer", params: { id: tab.id } };
  }
}

const sameTab = (a: Pick<WorkspaceTab, "kind" | "id">, b: Pick<WorkspaceTab, "kind" | "id">) =>
  a.kind === b.kind && a.id === b.id;

async function fetchTitle(kind: TabKind, id: string): Promise<string | null> {
  try {
    if (kind === "ticket") {
      const res = await call("frappe.client.get_value", {
        doctype: "HD Ticket",
        filters: id,
        fieldname: "subject",
      });
      return res?.subject ? `#${id} · ${res.subject}` : null;
    }
    if (kind === "contact") {
      const res = await call("frappe.client.get_value", {
        doctype: "Contact",
        filters: id,
        fieldname: "full_name",
      });
      return res?.full_name || null;
    }
    return id; // HD Customer is named by customer_name
  } catch {
    return null; // deleted or no permission: keep the fallback title
  }
}

function fallbackTitle(kind: TabKind, id: string) {
  return kind === "ticket" ? `#${id}` : id;
}

export const useWorkspaceTabsStore = defineStore("workspaceTabs", () => {
  const tabs = userStorage<WorkspaceTab[]>("kd_workspace_tabs", []);

  function open(kind: TabKind, id: string) {
    if (tabs.value.some((t) => sameTab(t, { kind, id }))) return;
    const tab: WorkspaceTab = { kind, id, title: fallbackTitle(kind, id) };
    tabs.value = [...tabs.value, tab].slice(-MAX_TABS);
    fetchTitle(kind, id).then((title) => title && rename(kind, id, title));
  }

  function rename(kind: TabKind, id: string, title: string) {
    tabs.value = tabs.value.map((t) =>
      sameTab(t, { kind, id }) ? { ...t, title } : t
    );
  }

  /** Removes a tab and returns the tab to show instead (its right neighbour,
   * else its left one), or null when no tabs are left. */
  function close(kind: TabKind, id: string): WorkspaceTab | null {
    const index = tabs.value.findIndex((t) => sameTab(t, { kind, id }));
    if (index === -1) return null;
    const next = tabs.value[index + 1] ?? tabs.value[index - 1] ?? null;
    tabs.value = tabs.value.filter((_, i) => i !== index);
    return next;
  }

  function closeAll() {
    tabs.value = [];
  }

  return { tabs, open, rename, close, closeAll };
});
