"use client";

import { CaretLeft, CaretRight } from "@phosphor-icons/react";
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "../select";
import { Button } from "../../ui/button";
import { cn } from "../../../lib/utils";
import { useEffect, useState } from 'react';

export type TablePaginationItem =
  | { type: "page"; index: number }
  | { type: "ellipsis"; direction: "previous" | "next"; target: number };

/**
 * ReUI Data Grid page window: first page, a run around the current one, last
 * page, and a clickable ellipsis for each hidden stretch.
 */
export function getTablePaginationItems(
  pageIndex: number,
  pageCount: number,
  limit = 5,
): TablePaginationItem[] {
  const sibling = Math.max(0, Math.floor(((Math.floor(limit) || 3) - 3) / 2));
  const pages = sibling * 2 + 3;

  if (pageCount <= pages + 2) {
    return Array.from({ length: pageCount }, (_, index) => ({
      type: "page" as const,
      index,
    }));
  }

  if (pageIndex <= sibling + 2) {
    return [
      ...Array.from({ length: pages }, (_, index) => ({
        type: "page" as const,
        index,
      })),
      { type: "ellipsis", direction: "next", target: pages },
      { type: "page", index: pageCount - 1 },
    ];
  }

  if (pageIndex >= pageCount - sibling - 3) {
    return [
      { type: "page", index: 0 },
      {
        type: "ellipsis",
        direction: "previous",
        target: pageCount - pages - 1,
      },
      ...Array.from({ length: pages }, (_, offset) => ({
        type: "page" as const,
        index: pageCount - pages + offset,
      })),
    ];
  }

  return [
    { type: "page", index: 0 },
    {
      type: "ellipsis",
      direction: "previous",
      target: pageIndex - sibling - 1,
    },
    ...Array.from({ length: sibling * 2 + 1 }, (_, offset) => ({
      type: "page" as const,
      index: pageIndex - sibling + offset,
    })),
    { type: "ellipsis", direction: "next", target: pageIndex + sibling + 1 },
    { type: "page", index: pageCount - 1 },
  ];
}

export interface TablePaginationProps {
  pageIndex: number;
  pageSize: number;
  pageCount: number;
  recordCount: number;
  sizes?: number[];
  moreLimit?: number;
  rowsPerPageLabel?: string;
  info?: string;
  previousPageLabel?: string;
  nextPageLabel?: string;
  ellipsisText?: string;
  onPageIndexChange: (index: number) => void;
  onPageSizeChange: (size: number) => void;
  className?: string;
}

export function TablePagination({
  pageIndex,
  pageSize,
  pageCount,
  recordCount,
  sizes = [5, 10, 25, 50, 100],
  moreLimit = 5,
  rowsPerPageLabel = "Rows per page",
  info = "{from} - {to} of {count}",
  previousPageLabel = "Go to previous page",
  nextPageLabel = "Go to next page",
  ellipsisText = "...",
  onPageIndexChange,
  onPageSizeChange,
  className,
}: TablePaginationProps) {
  const [jump, setJump] = useState(String(pageIndex + 1));
  useEffect(() => setJump(String(pageIndex + 1)), [pageIndex]);
  const from = recordCount === 0 ? 0 : pageIndex * pageSize + 1;
  const to = Math.min((pageIndex + 1) * pageSize, recordCount);
  const paginationInfo = info
    .replaceAll("{from}", String(from))
    .replaceAll("{to}", String(to))
    .replaceAll("{count}", String(recordCount));
  const items = getTablePaginationItems(pageIndex, pageCount, moreLimit);
  const canPrevious = pageIndex > 0;
  const canNext = pageIndex < pageCount - 1;

  return (
    <div
      className={cn(
        "relative z-20 flex flex-wrap items-center justify-between gap-2 border-border border-t px-3 py-1 text-xs",
        className,
      )}
    >
      <div className="flex items-center gap-2">
        <span className="whitespace-nowrap text-muted-foreground text-xs">
          {rowsPerPageLabel}
        </span>
        <Select
          value={String(pageSize)}
          onValueChange={(value) => {
            if (value == null) return;
            onPageSizeChange(Number(value));
          }}
          className="w-16"
          side="top"
        >
          <SelectTrigger aria-label={rowsPerPageLabel} className="h-6 min-h-6 rounded-md px-2 py-0 text-xs gap-1 [&_svg]:size-3">
            <SelectValue />
          </SelectTrigger>
          <SelectContent>
            {sizes.map((size) => (
              <SelectItem
                key={size}
                value={String(size)}
                className={
                  pageSize === size
                    ? "bg-muted text-foreground text-xs py-1"
                    : "text-xs py-1"
                }
              >
                {size}
              </SelectItem>
            ))}
          </SelectContent>
        </Select>
      </div>

      <div className="flex flex-wrap items-center justify-end gap-2">
        <span className="whitespace-nowrap text-muted-foreground text-xs tabular-nums">
          {paginationInfo}
        </span>
        {pageCount > 1 ? (
          <nav
            aria-label="Pagination"
            className="flex items-center gap-0.5"
          >
            <Button
              type="button"
              variant="ghost"
              size="icon-xs"
              aria-label={previousPageLabel}
              disabled={!canPrevious}
              onClick={() => onPageIndexChange(pageIndex - 1)}
            >
              <CaretLeft />
            </Button>
            {items.map((item) =>
              item.type === "page" ? (
                <Button
                  key={`page-${item.index}`}
                  type="button"
                  variant={pageIndex === item.index ? "outline" : "ghost"}
                  size="icon-xs"
                  aria-label={`Page ${item.index + 1}`}
                  aria-current={pageIndex === item.index ? "page" : undefined}
                  className="min-w-6 w-auto px-1 text-xs"
                  onClick={() => {
                    if (pageIndex !== item.index) onPageIndexChange(item.index);
                  }}
                >
                  {item.index + 1}
                </Button>
              ) : (
                <Button
                  key={`ellipsis-${item.direction}-${item.target}`}
                  type="button"
                  variant="ghost"
                  size="icon-xs"
                  aria-label={
                    item.direction === "next"
                      ? "Next pages"
                      : "Previous pages"
                  }
                  className="min-w-6 w-auto px-1 text-xs text-muted-foreground"
                  onClick={() => onPageIndexChange(item.target)}
                >
                  {ellipsisText}
                </Button>
              ),
            )}
            <Button
              type="button"
              variant="ghost"
              size="icon-xs"
              aria-label={nextPageLabel}
              disabled={!canNext}
              onClick={() => onPageIndexChange(pageIndex + 1)}
            >
              <CaretRight />
            </Button>
            <form onSubmit={event => {
              event.preventDefault();
              const requested = Number(jump);
              if (Number.isSafeInteger(requested) && requested >= 1 && requested <= pageCount) onPageIndexChange(requested - 1);
            }} className="ml-1 flex items-center gap-1">
              <label className="text-muted-foreground text-xs">跳至页
                <input type="number" aria-label="跳至页" min={1} max={pageCount} value={jump}
                  onChange={event=>setJump(event.target.value)}
                  className="ml-1 h-6 w-12 rounded-md border border-border bg-transparent px-1 text-xs text-foreground" />
              </label>
              <Button type="submit" variant="ghost" size="icon-xs" aria-label="跳转到指定页"><CaretRight /></Button>
            </form>
          </nav>
        ) : null}
      </div>
    </div>
  );
}
