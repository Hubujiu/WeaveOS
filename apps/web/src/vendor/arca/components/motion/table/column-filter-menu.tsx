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
}: {
  columnKey: string;
  header: ReactNode;
  sort: SortState | null;
  onSort: (key: string, direction: SortDirection) => void;
  reduce?: boolean;
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
    const onKey = (event: KeyboardEvent) => {
      if (event.key === "Escape") setOpen(false);
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
        aria-label={`筛选 ${columnKey}`}
        aria-haspopup="menu"
        aria-expanded={open}
        onPointerDown={(event) => event.stopPropagation()}
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
        <Funnel size={14} weight={active ? "fill" : "light"} />
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
                    }}
                  />
                  <SortMenuItem
                    icon={<SortDescending size={16} />}
                    label="降序"
                    active={active && sort?.direction === "desc"}
                    onSelect={() => {
                      onSort(columnKey, "desc");
                      setOpen(false);
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
          "flex w-full items-center gap-2 rounded-lg px-2.5 py-1.5 text-left text-sm outline-none transition-colors",
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
