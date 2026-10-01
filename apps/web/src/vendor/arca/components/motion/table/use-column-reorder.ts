import {
  type PointerEvent as ReactPointerEvent,
  useCallback,
  useMemo,
  useRef,
  useState,
} from "react";
import { capturePointer, releasePointer } from "../../../lib/touch";
import type { HeaderCellRefs, TableColumn } from "./types";

const DRAG_THRESHOLD = 5;

export function useColumnReorder<T>({
  columns,
  thRefs,
  onColumnOrderChange,
}: {
  columns: TableColumn<T>[];
  thRefs: HeaderCellRefs;
  onColumnOrderChange?: (keys: string[]) => void;
}) {
  const [order, setOrder] = useState<string[]>(() =>
    columns.map((c) => c.key),
  );
  const [dragKey, setDragKey] = useState<string | null>(null);
  const [dropIndex, setDropIndex] = useState<number | null>(null);
  const dragRef = useRef<{
    key: string;
    originX: number;
    moved: boolean;
  } | null>(null);

  // Apply the current order, tolerating columns added/removed at runtime. New
  // columns are placed at their position in `columns` (after their left
  // neighbor), not appended — so an inserted column lands where it was added.
  const orderedColumns = useMemo(() => {
    const byKey = new Map(columns.map((c) => [c.key, c]));
    const resultKeys = order.filter((k) => byKey.has(k));
    const present = new Set(resultKeys);
    columns.forEach((column, i) => {
      if (present.has(column.key)) return;
      let at = resultKeys.length;
      if (i === 0) {
        at = 0;
      } else {
        const idx = resultKeys.indexOf(columns[i - 1].key);
        at = idx === -1 ? i : idx + 1;
      }
      resultKeys.splice(at, 0, column.key);
      present.add(column.key);
    });
    return resultKeys
      .map((k) => byKey.get(k))
      .filter((c): c is TableColumn<T> => c !== undefined);
  }, [order, columns]);

  const dropIndexFor = useCallback(
    (clientX: number) => {
      for (let i = 0; i < orderedColumns.length; i++) {
        const rect =
          thRefs.current[orderedColumns[i].key]?.getBoundingClientRect();
        if (rect && clientX < rect.left + rect.width / 2) return i;
      }
      return orderedColumns.length;
    },
    [orderedColumns, thRefs],
  );

  const startReorder = useCallback((key: string, e: ReactPointerEvent) => {
    if (e.button !== 0) return;
    e.preventDefault();
    e.stopPropagation();
    dragRef.current = { key, originX: e.clientX, moved: false };
    capturePointer(e.currentTarget, e.pointerId);
  }, []);

  const moveReorder = useCallback(
    (e: ReactPointerEvent) => {
      const drag = dragRef.current;
      if (!drag) return;
      if (!drag.moved) {
        if (Math.abs(e.clientX - drag.originX) < DRAG_THRESHOLD) return;
        drag.moved = true;
        setDragKey(drag.key);
      }
      setDropIndex(dropIndexFor(e.clientX));
    },
    [dropIndexFor],
  );

  const endReorder = useCallback(
    (e: ReactPointerEvent) => {
      const drag = dragRef.current;
      const moved = Boolean(drag?.moved);
      const key = drag?.key ?? null;
      dragRef.current = null;
      releasePointer(e.currentTarget, e.pointerId);
      if (moved && key) {
        const keys = orderedColumns.map((c) => c.key);
        const from = keys.indexOf(key);
        if (from !== -1) {
          const without = keys.filter((_, i) => i !== from);
          let to = dropIndexFor(e.clientX);
          if (from < to) to--;
          without.splice(Math.max(0, to), 0, key);
          setOrder(without);
          onColumnOrderChange?.(without);
        }
      }
      setDragKey(null);
      setDropIndex(null);
      return moved;
    },
    [dropIndexFor, orderedColumns, onColumnOrderChange],
  );

  return {
    orderedColumns,
    dragKey,
    dropIndex,
    startReorder,
    moveReorder,
    endReorder,
  };
}
