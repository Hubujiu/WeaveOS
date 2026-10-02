"use client";

import { Funnel, SortAscending, SortDescending } from "@phosphor-icons/react";
import { motion, useReducedMotion } from "motion/react";
import {
  useEffect,
  useId,
  useLayoutEffect,
  useState,
  type ReactNode,
} from "react";
import { createPortal } from "react-dom";
import {
  DROPDOWN_ITEM_VARIANTS,
  DROPDOWN_LIST_VARIANTS,
  DropdownPanel,
  type DropdownPlacement,
} from "../dropdown-panel";
import { cn } from "../../../lib/utils";
import type { SortDirection, SortState } from "./types";

const MENU_WIDTH = 168;

export function ColumnFilterMenu({
  columnKey,
  header,
  sort,
  onSort,
  reduce: reduceProp,
  sortOnly = false,
}: {
  columnKey: string;
  header: ReactNode;
  sort: SortState | null;
  onSort: (key: string, direction: SortDirection) => void;
  reduce?: boolean;
  sortOnly?: boolean;
}) {
  const reduce = useReducedMotion() ?? reduceProp ?? false;
  const triggerId = useId();
  const [open, setOpen] = useState(false);
  const [mounted, setMounted] = useState(false);
  const [placement, setPlacement] = useState<DropdownPlacement>("bottom");
  const [anchor, setAnchor] = useState<{ top: number; left: number } | null>(
    null,
  );
  const active = sort?.key === columnKey;

  const focusMenu = (position: "first" | "last") => {
    requestAnimationFrame(() => {
      const items = document.getElementById(`${triggerId}-panel`)?.querySelectorAll<HTMLButtonElement>('[role="menuitem"]');
      items?.[position === "last" ? items.length - 1 : 0]?.focus();
    });
  };

  useEffect(() => {
    setMounted(true);
  }, []);

  useLayoutEffect(() => {
    if (!open) return;
    const trigger = document.getElementById(triggerId);
    if (!trigger) return;
    const rect = trigger.getBoundingClientRect();
    setAnchor({
      top: rect.bottom,
      left: Math.min(
        Math.max(8, rect.right - MENU_WIDTH),
        window.innerWidth - MENU_WIDTH - 8,
      ),
    });
  }, [open, triggerId]);

  useEffect(() => {
    if (!open) return;
    // A focus/scrollIntoView event can already be queued when click opens the
    // menu. Its geometry is captured after that scroll; only a later position
    // change should dismiss the correctly anchored menu.
    const openingScroll = new Map<HTMLElement, {left:number;top:number}>();
    for (let node=document.getElementById(triggerId)?.parentElement;node;node=node.parentElement) {
      openingScroll.set(node,{left:node.scrollLeft,top:node.scrollTop});
    }
    const onKey = (event: KeyboardEvent) => {
      if (event.defaultPrevented) return;
      const trigger = document.getElementById(triggerId);
      if (event.key === "Escape") {
        event.preventDefault();
        event.stopPropagation();
        setOpen(false);
        trigger?.focus();
        return;
      }
      const panel = document.getElementById(`${triggerId}-panel`);
      if (!(event.target instanceof Node) || !panel?.contains(event.target)) return;
      if (event.key === "Tab") {
        setOpen(false);
        trigger?.focus();
        return;
      }
      const items = [...panel.querySelectorAll<HTMLButtonElement>('[role="menuitem"]')];
      if (!items.length || !["ArrowDown", "ArrowUp", "Home", "End"].includes(event.key)) return;
      event.preventDefault();
      const index = items.indexOf(document.activeElement as HTMLButtonElement);
      const next = event.key === "Home" ? 0 : event.key === "End" ? items.length - 1
        : (index + (event.key === "ArrowDown" ? 1 : -1) + items.length) % items.length;
      items[next]?.focus();
    };
    const onPointer = (event: PointerEvent) => {
      const trigger = document.getElementById(triggerId);
      const panel = document.getElementById(`${triggerId}-panel`);
      const target = event.target as Node;
      if (trigger?.contains(target) || panel?.contains(target)) return;
      setOpen(false);
    };
    const onClose = (event: Event) => {
      const panel = document.getElementById(`${triggerId}-panel`);
      if (event.type === "scroll" && event.target instanceof Node && panel?.contains(event.target)) return;
      if (event.type === "scroll" && event.target instanceof HTMLElement) {
        const position=openingScroll.get(event.target);
        if(position&&position.left===event.target.scrollLeft&&position.top===event.target.scrollTop)return;
      }
      setOpen(false);
    };
    window.addEventListener("keydown", onKey);
    window.addEventListener("pointerdown", onPointer);
    window.addEventListener("scroll", onClose, true);
    window.addEventListener("resize", onClose);
    return () => {
      window.removeEventListener("keydown", onKey);
      window.removeEventListener("pointerdown", onPointer);
      window.removeEventListener("scroll", onClose, true);
      window.removeEventListener("resize", onClose);
    };
  }, [open, triggerId]);

  return (
    <div className="relative flex h-full min-w-0 flex-1 items-center">
      <span className="min-w-0 flex-1 truncate px-1">{header}</span>
      <button
        id={triggerId}
        type="button"
        aria-label={`${sortOnly ? '排序' : '筛选'} ${columnKey}`}
        aria-haspopup="menu"
        aria-expanded={open}
        aria-controls={mounted ? `${triggerId}-panel` : undefined}
        onPointerDown={(event) => event.stopPropagation()}
        onKeyDown={(event) => {
          if (!["ArrowDown", "ArrowUp", "Home", "End"].includes(event.key)) return;
          event.preventDefault();
          event.stopPropagation();
          setOpen(true);
          focusMenu(event.key === "ArrowUp" || event.key === "End" ? "last" : "first");
        }}
        onClick={(event) => {
          event.stopPropagation();
          if (!open) {
            const trigger = document.getElementById(triggerId);
            if (trigger) {
              const rect = trigger.getBoundingClientRect();
              setAnchor({
                top: rect.bottom,
                left: Math.min(
                  Math.max(8, rect.right - MENU_WIDTH),
                  window.innerWidth - MENU_WIDTH - 8,
                ),
              });
            }
            setOpen(true);
            if (event.detail === 0) focusMenu("first");
            return;
          }
          setOpen(false);
        }}
        className={cn(
          "relative z-30 mr-1.5 grid size-6 shrink-0 place-items-center rounded-md transition-all duration-150",
          active || open
            ? "opacity-100 text-foreground bg-background/80 shadow-xs"
            : "opacity-0 text-muted-foreground/70 hover:bg-muted hover:text-foreground group-hover:opacity-100 group-focus-within:opacity-100",
        )}
      >
        {sortOnly ? (active && sort?.direction === 'desc' ? <SortDescending size={14} /> : <SortAscending size={14} />) : <Funnel size={14} weight={active ? "fill" : "light"} />}
      </button>
      {mounted
        ? createPortal(
            <div
              className="arca-source relative"
              style={{
                position: "fixed",
                top: anchor?.top ?? -9999,
                left: anchor?.left ?? 0,
                width: MENU_WIDTH,
                height: 0,
                zIndex: 60,
              }}
            >
              <DropdownPanel
                open={open}
                reduce={reduce}
                placement={placement}
                setPlacement={setPlacement}
                triggerId={triggerId}
                listId={`${triggerId}-panel`}
                labelledBy={triggerId}
                role="menu"
              >
                <motion.div
                  variants={reduce ? undefined : DROPDOWN_LIST_VARIANTS}
                  initial={false}
                  animate={open ? "show" : "hidden"}
                  className="p-1"
                >
                  <SortMenuItem
                    icon={<SortAscending size={16} />}
                    label="升序"
                    active={active && sort?.direction === "asc"}
                    onSelect={() => {
                      onSort(columnKey, "asc");
                      setOpen(false);
                      document.getElementById(triggerId)?.focus();
                    }}
                  />
                  <SortMenuItem
                    icon={<SortDescending size={16} />}
                    label="降序"
                    active={active && sort?.direction === "desc"}
                    onSelect={() => {
                      onSort(columnKey, "desc");
                      setOpen(false);
                      document.getElementById(triggerId)?.focus();
                    }}
                  />
                </motion.div>
              </DropdownPanel>
            </div>,
            document.body,
          )
        : null}
    </div>
  );
}

function SortMenuItem({
  icon,
  label,
  active,
  onSelect,
}: {
  icon: ReactNode;
  label: string;
  active: boolean;
  onSelect: () => void;
}) {
  return (
    <motion.div variants={DROPDOWN_ITEM_VARIANTS}>
      <button
        type="button"
        role="menuitem"
        aria-checked={active}
        onClick={onSelect}
        className={cn(
          "flex w-full items-center gap-2 rounded-lg px-2.5 py-1.5 text-left text-sm outline-none transition-colors focus-visible:bg-muted focus-visible:text-foreground focus-visible:outline-solid focus-visible:outline-2 focus-visible:outline-ring focus-visible:-outline-offset-2",
          active
            ? "bg-muted text-foreground"
            : "text-muted-foreground hover:bg-muted hover:text-foreground",
        )}
      >
        {icon}
        {label}
      </button>
    </motion.div>
  );
}
