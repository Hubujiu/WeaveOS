"use client";

import { cn } from "../../../lib/utils";
import type { TableColumn } from "./types";
import { alignText } from "./utils";

export function SkeletonRows<T>({
  count,
  columns,
  selectable,
  rowHeight,
}: {
  count: number;
  columns: TableColumn<T>[];
  selectable: boolean;
  rowHeight: number;
}) {
  return (
    <>
      {Array.from({ length: count }, (_, r) => (
        // biome-ignore lint/suspicious/noArrayIndexKey: static placeholder rows
        <tr key={r} style={{ height: rowHeight }} className="border-border/60 border-b">
          {selectable ? <td className="border-border border-r" /> : null}
          {columns.map((column, index) => (
            <td
              key={column.key}
              className={cn(
                "px-4",
                alignText(column.align),
                index < columns.length - 1 && "border-border border-r",
              )}
            >
              <div
                className={cn(
                  "h-3 animate-pulse rounded-full bg-muted",
                  column.align === "right" ? "ml-auto w-10" : "w-2/3",
                )}
              />
            </td>
          ))}
        </tr>
      ))}
    </>
  );
}
