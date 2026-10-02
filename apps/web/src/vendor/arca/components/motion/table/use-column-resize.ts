import {
  type PointerEvent as ReactPointerEvent,
  useCallback,
  useRef,
  useState,
} from "react";
import { capturePointer, releasePointer } from "../../../lib/touch";
import type { HeaderCellRefs, TableColumn } from "./types";

export function useColumnResize<T>({
  orderedColumns,
  thRefs,
  minColumnWidth,
  onColumnResize,
  columnWidths,
  onColumnWidthsChange,
}: {
  orderedColumns: TableColumn<T>[];
  thRefs: HeaderCellRefs;
  minColumnWidth: number;
  onColumnResize?: (key: string, width: number) => void;
  columnWidths?: Record<string, number>;
  onColumnWidthsChange?: (widths: Record<string, number>) => void;
}) {
  const resizeRef = useRef<{
    key: string;
    startX: number;
    startWidths: Record<string, number>;
    keys: string[];
  } | null>(null);
  const widthsRef = useRef<Record<string, number>>({});
  const [liveWidths, setWidths] = useState<Record<string, number>>({});
  const [resizing, setResizing] = useState(false);
  const widths = resizing ? liveWidths : columnWidths ?? liveWidths;

  const commitWidths = useCallback((next: Record<string, number>) => {
    widthsRef.current = next;
    setWidths(next);
  }, []);

  const startResize = useCallback(
    (key: string, e: ReactPointerEvent) => {
      e.preventDefault();
      e.stopPropagation();
      const snapshot = { ...widths };
      for (const column of orderedColumns) {
        if (snapshot[column.key] == null) {
          const measured = thRefs.current[column.key]?.getBoundingClientRect()
            .width;
          snapshot[column.key] = measured
            ? Math.round(measured)
            : minColumnWidth;
        }
      }
      resizeRef.current = {
        key,
        startX: e.clientX,
        startWidths: snapshot,
        keys: orderedColumns.map((column) => column.key),
      };
      setResizing(true);
      commitWidths(snapshot);
      capturePointer(e.currentTarget, e.pointerId);
    },
    [commitWidths, minColumnWidth, orderedColumns, thRefs, widths],
  );

  const moveResize = useCallback(
    (e: ReactPointerEvent) => {
      const state = resizeRef.current;
      if (!state) return;
      const index = state.keys.indexOf(state.key);
      if (index < 0) return;
      const delta = Math.round(e.clientX - state.startX);
      const next = { ...state.startWidths };
      if (delta > 0) {
        let remaining = delta;
        const stealAt: number[] = [];
        for (let i = index + 1; i < state.keys.length; i++) stealAt.push(i);
        if (stealAt.length === 0) {
          for (let i = index - 1; i >= 0; i--) stealAt.push(i);
        }
        for (const i of stealAt) {
          if (remaining <= 0) break;
          const available = next[state.keys[i]] - minColumnWidth;
          if (available <= 0) continue;
          const take = Math.min(available, remaining);
          next[state.keys[i]] -= take;
          remaining -= take;
        }
        next[state.key] = state.startWidths[state.key] + (delta - remaining);
      } else {
        const shrink = Math.min(-delta, next[state.key] - minColumnWidth);
        next[state.key] -= shrink;
        const neighbor =
          state.keys[index + 1] ?? (index > 0 ? state.keys[index - 1] : null);
        if (neighbor) next[neighbor] += shrink;
      }
      commitWidths(next);
    },
    [commitWidths, minColumnWidth],
  );

  const endResize = useCallback(
    (e: ReactPointerEvent) => {
      const state = resizeRef.current;
      resizeRef.current = null;
      setResizing(false);
      releasePointer(e.currentTarget, e.pointerId);
      if (state) {
        onColumnWidthsChange?.({...widthsRef.current});
        onColumnResize?.(
          state.key,
          widthsRef.current[state.key] ?? state.startWidths[state.key],
        );
      }
    },
    [onColumnResize, onColumnWidthsChange],
  );

  return { widths, startResize, moveResize, endResize };
}
