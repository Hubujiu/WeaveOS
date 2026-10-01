"use client";

import { ChevronDown } from "lucide-react";
import {
  motion,
  type Transition,
  type Variants,
} from "motion/react";
import {
  type ReactNode,
  type Ref,
  type AriaRole,
  useLayoutEffect,
  useRef,
  useState,
} from "react";
import { EASE_OUT } from "../../lib/ease";
import { cn } from "../../lib/utils";

export type DropdownPlacement = "bottom" | "top";

export const DROPDOWN_INSTANT: Transition = { duration: 0 };

export const DROPDOWN_CHEVRON: Transition = {
  type: "spring",
  duration: 0.4,
  bounce: 0.3,
};

export const DROPDOWN_LIST_VARIANTS: Variants = {
  hidden: {},
  show: { transition: { staggerChildren: 0.035, delayChildren: 0.05 } },
};

export const DROPDOWN_ITEM_VARIANTS: Variants = {
  hidden: { opacity: 0, y: -6, filter: "blur(3px)" },
  show: { opacity: 1, y: 0, filter: "blur(0px)" },
};

export function getDropdownTriggerMotion(
  open: boolean,
  placement: DropdownPlacement,
  reduce: boolean,
) {
  const isTop = placement === "top";
  const kf = open ? [0, 0, 12] : [12, 0, 12];
  const kfT: Transition = reduce
    ? { duration: 0 }
    : open
      ? { duration: 0.6, times: [0, 0.4, 1], ease: EASE_OUT }
      : { duration: 0.42, times: [0, 0.5, 1], ease: EASE_OUT };

  return {
    animate: {
      borderTopLeftRadius: isTop ? kf : 12,
      borderTopRightRadius: isTop ? kf : 12,
      borderBottomLeftRadius: isTop ? 12 : kf,
      borderBottomRightRadius: isTop ? 12 : kf,
    },
    transition: {
      borderTopLeftRadius: isTop ? kfT : DROPDOWN_INSTANT,
      borderTopRightRadius: isTop ? kfT : DROPDOWN_INSTANT,
      borderBottomLeftRadius: isTop ? DROPDOWN_INSTANT : kfT,
      borderBottomRightRadius: isTop ? DROPDOWN_INSTANT : kfT,
    },
  };
}

export function DropdownChevron({
  open,
  reduce,
  className,
}: {
  open: boolean;
  reduce: boolean;
  className?: string;
}) {
  return (
    <motion.span
      aria-hidden
      animate={{ rotate: open ? 180 : 0 }}
      transition={reduce ? { duration: 0 } : DROPDOWN_CHEVRON}
      className={cn("shrink-0 text-muted-foreground", className)}
    >
      <ChevronDown className="h-4 w-4" />
    </motion.span>
  );
}

export interface DropdownPanelProps {
  open: boolean;
  reduce: boolean;
  placement: DropdownPlacement;
  setPlacement: (placement: DropdownPlacement) => void;
  triggerId: string;
  listId?: string;
  labelledBy?: string;
  role?: AriaRole;
  className?: string;
  children: ReactNode;
  contentRef?: Ref<HTMLDivElement>;
  /** Prefer opening in this direction when there is room. */
  preferredPlacement?: DropdownPlacement;
}

export function DropdownPanel({
  open,
  reduce,
  placement,
  setPlacement,
  triggerId,
  listId,
  labelledBy,
  role,
  className,
  children,
  contentRef,
  preferredPlacement,
}: DropdownPanelProps) {
  const innerRef = useRef<HTMLDivElement>(null);
  const [height, setHeight] = useState(0);
  const isTop = placement === "top";
  const nearGap = open ? 8 : 0;
  const nearRadius = open ? 12 : 0;

  const gapT: Transition = open
    ? { type: "spring", duration: 0.6, bounce: 0.5, delay: 0.12 }
    : { type: "spring", duration: 0.3, bounce: 0.1 };
  const radiusT: Transition = open
    ? { duration: 0.3, ease: EASE_OUT, delay: 0.14 }
    : { duration: 0.16, ease: EASE_OUT };

  useLayoutEffect(() => {
    const node = innerRef.current;
    if (!node) return;
    const measure = () => setHeight(node.offsetHeight);
    measure();
    const observer = new ResizeObserver(measure);
    observer.observe(node);
    return () => observer.disconnect();
  });

  useLayoutEffect(() => {
    if (!open) return;
    const trigger = document.getElementById(triggerId);
    const node = innerRef.current;
    if (!trigger || !node) return;
    const rect = trigger.getBoundingClientRect();
    const h = node.offsetHeight;
    const below = window.innerHeight - rect.bottom;
    const above = rect.top;
    if (preferredPlacement === "top") {
      setPlacement(above > h + 16 ? "top" : "bottom");
      return;
    }
    setPlacement(below < h + 16 && above > below ? "top" : "bottom");
  }, [open, preferredPlacement, setPlacement, triggerId]);

  return (
    <motion.div
      ref={contentRef}
      id={listId}
      role={role}
      aria-labelledby={labelledBy}
      aria-hidden={!open}
      inert={!open}
      initial={false}
      animate={
        reduce
          ? { opacity: open ? 1 : 0, height: open ? height : 0 }
          : {
              opacity: open ? 1 : 0,
              height: open ? height : 0,
              marginTop: isTop ? 0 : nearGap,
              marginBottom: isTop ? nearGap : 0,
              borderTopLeftRadius: isTop ? 12 : nearRadius,
              borderTopRightRadius: isTop ? 12 : nearRadius,
              borderBottomLeftRadius: isTop ? nearRadius : 12,
              borderBottomRightRadius: isTop ? nearRadius : 12,
            }
      }
      transition={
        reduce
          ? { duration: 0.12 }
          : {
              opacity: open
                ? { duration: 0.18 }
                : { duration: 0.16, delay: 0.12 },
              height: open
                ? { type: "spring", duration: 0.42, bounce: 0.14 }
                : { duration: 0.26, ease: EASE_OUT, delay: 0.14 },
              marginTop: isTop ? DROPDOWN_INSTANT : gapT,
              marginBottom: isTop ? gapT : DROPDOWN_INSTANT,
              borderTopLeftRadius: isTop ? DROPDOWN_INSTANT : radiusT,
              borderTopRightRadius: isTop ? DROPDOWN_INSTANT : radiusT,
              borderBottomLeftRadius: isTop ? radiusT : DROPDOWN_INSTANT,
              borderBottomRightRadius: isTop ? radiusT : DROPDOWN_INSTANT,
            }
      }
      style={{
        transformOrigin: isTop ? "bottom" : "top",
        overflow: "hidden",
        pointerEvents: open ? "auto" : "none",
      }}
      className={cn(
        "absolute left-0 right-0 z-20 rounded-xl border border-border bg-background shadow-lg",
        isTop ? "bottom-full" : "top-full",
        className,
      )}
    >
      <div ref={innerRef}>{children}</div>
    </motion.div>
  );
}
