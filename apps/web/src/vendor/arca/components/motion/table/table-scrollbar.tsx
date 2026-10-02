"use client";

import {
  type PointerEvent as ReactPointerEvent,
  type RefObject,
  useCallback,
  useEffect,
  useRef,
  useState,
} from "react";
import { cn } from "../../../lib/utils";

interface ScrollbarThumbState {
  size: number;
  offset: number;
  visible: boolean;
}

interface TableScrollbarProps {
  scrollRef: RefObject<HTMLDivElement | null>;
  headerHeight?: number;
  className?: string;
}

export function TableScrollbar({
  scrollRef,
  headerHeight = 0,
  className,
}: TableScrollbarProps) {
  const [vertical, setVertical] = useState<ScrollbarThumbState>({
    size: 0,
    offset: 0,
    visible: false,
  });
  const [horizontal, setHorizontal] = useState<ScrollbarThumbState>({
    size: 0,
    offset: 0,
    visible: false,
  });

  const [isHovered, setIsHovered] = useState(false);
  const [isScrolling, setIsScrolling] = useState(false);
  const [isDraggingVertical, setIsDraggingVertical] = useState(false);
  const [isDraggingHorizontal, setIsDraggingHorizontal] = useState(false);

  const scrollTimer = useRef<ReturnType<typeof setTimeout> | null>(null);
  const vTrackRef = useRef<HTMLDivElement>(null);
  const hTrackRef = useRef<HTMLDivElement>(null);

  const updateScrollbars = useCallback(() => {
    const el = scrollRef.current;
    if (!el) return;

    const {
      clientHeight,
      scrollHeight,
      scrollTop,
      clientWidth,
      scrollWidth,
      scrollLeft,
    } = el;

    // Vertical: restricted to data rows (below the header row)
    if (scrollHeight > clientHeight + 1) {
      const topOffset = headerHeight > 0 ? headerHeight + 2 : 2;
      const trackHeight = clientHeight - topOffset - 2; // topOffset at top, 2px at bottom
      const rawSize = (clientHeight / scrollHeight) * trackHeight;
      const thumbHeight = Math.max(20, Math.min(rawSize, trackHeight));
      const maxScrollTop = scrollHeight - clientHeight;
      const maxThumbTop = trackHeight - thumbHeight;
      const thumbTop =
        maxScrollTop > 0 ? (scrollTop / maxScrollTop) * maxThumbTop : 0;

      setVertical({
        size: thumbHeight,
        offset: thumbTop,
        visible: true,
      });
    } else {
      setVertical((prev) => (prev.visible ? { ...prev, visible: false } : prev));
    }

    // Horizontal
    if (scrollWidth > clientWidth + 1) {
      const trackWidth = clientWidth - 4; // 2px margin left & right
      const rawSize = (clientWidth / scrollWidth) * trackWidth;
      const thumbWidth = Math.max(20, Math.min(rawSize, trackWidth));
      const maxScrollLeft = scrollWidth - clientWidth;
      const maxThumbLeft = trackWidth - thumbWidth;
      const thumbLeft =
        maxScrollLeft > 0 ? (scrollLeft / maxScrollLeft) * maxThumbLeft : 0;

      setHorizontal({
        size: thumbWidth,
        offset: thumbLeft,
        visible: true,
      });
    } else {
      setHorizontal((prev) =>
        prev.visible ? { ...prev, visible: false } : prev,
      );
    }
  }, [scrollRef, headerHeight]);

  // Listen to scroll & resize
  useEffect(() => {
    const el = scrollRef.current;
    if (!el) return;

    const onScroll = () => {
      updateScrollbars();
      setIsScrolling(true);
      if (scrollTimer.current) clearTimeout(scrollTimer.current);
      scrollTimer.current = setTimeout(() => {
        setIsScrolling(false);
      }, 800);
    };

    el.addEventListener("scroll", onScroll, { passive: true });

    // ResizeObserver
    const observer = new ResizeObserver(() => {
      updateScrollbars();
    });
    observer.observe(el);
    if (el.firstElementChild) {
      observer.observe(el.firstElementChild);
    }

    // Initial update
    updateScrollbars();

    return () => {
      el.removeEventListener("scroll", onScroll);
      observer.disconnect();
      if (scrollTimer.current) clearTimeout(scrollTimer.current);
    };
  }, [scrollRef, updateScrollbars]);

  // Hover detection on parent container
  useEffect(() => {
    const el = scrollRef.current?.parentElement;
    if (!el) return;

    const onEnter = () => setIsHovered(true);
    const onLeave = () => setIsHovered(false);

    el.addEventListener("mouseenter", onEnter);
    el.addEventListener("mouseleave", onLeave);

    return () => {
      el.removeEventListener("mouseenter", onEnter);
      el.removeEventListener("mouseleave", onLeave);
    };
  }, [scrollRef]);

  // Vertical thumb drag
  const handleVerticalThumbPointerDown = (e: ReactPointerEvent) => {
    e.preventDefault();
    e.stopPropagation();

    const el = scrollRef.current;
    if (!el) return;

    const startY = e.clientY;
    const startScrollTop = el.scrollTop;
    const topOffset = headerHeight > 0 ? headerHeight + 2 : 2;
    const trackHeight = el.clientHeight - topOffset - 2;
    const maxScrollTop = el.scrollHeight - el.clientHeight;
    const maxThumbTop = trackHeight - vertical.size;

    if (maxThumbTop <= 0) return;

    setIsDraggingVertical(true);
    const prevUserSelect = document.body.style.userSelect;
    document.body.style.userSelect = "none";

    const onPointerMove = (moveEvent: PointerEvent) => {
      const deltaY = moveEvent.clientY - startY;
      el.scrollTop = startScrollTop + (deltaY / maxThumbTop) * maxScrollTop;
    };

    const onPointerUp = () => {
      setIsDraggingVertical(false);
      document.body.style.userSelect = prevUserSelect;
      window.removeEventListener("pointermove", onPointerMove);
      window.removeEventListener("pointerup", onPointerUp);
      window.removeEventListener("pointercancel", onPointerUp);
    };

    window.addEventListener("pointermove", onPointerMove);
    window.addEventListener("pointerup", onPointerUp);
    window.addEventListener("pointercancel", onPointerUp);
  };

  // Horizontal thumb drag
  const handleHorizontalThumbPointerDown = (e: ReactPointerEvent) => {
    e.preventDefault();
    e.stopPropagation();

    const el = scrollRef.current;
    if (!el) return;

    const startX = e.clientX;
    const startScrollLeft = el.scrollLeft;
    const trackWidth = el.clientWidth - 4;
    const maxScrollLeft = el.scrollWidth - el.clientWidth;
    const maxThumbLeft = trackWidth - horizontal.size;

    if (maxThumbLeft <= 0) return;

    setIsDraggingHorizontal(true);
    const prevUserSelect = document.body.style.userSelect;
    document.body.style.userSelect = "none";

    const onPointerMove = (moveEvent: PointerEvent) => {
      const deltaX = moveEvent.clientX - startX;
      el.scrollLeft = startScrollLeft + (deltaX / maxThumbLeft) * maxScrollLeft;
    };

    const onPointerUp = () => {
      setIsDraggingHorizontal(false);
      document.body.style.userSelect = prevUserSelect;
      window.removeEventListener("pointermove", onPointerMove);
      window.removeEventListener("pointerup", onPointerUp);
      window.removeEventListener("pointercancel", onPointerUp);
    };

    window.addEventListener("pointermove", onPointerMove);
    window.addEventListener("pointerup", onPointerUp);
    window.addEventListener("pointercancel", onPointerUp);
  };

  // Click on track to jump
  const handleVerticalTrackPointerDown = (e: ReactPointerEvent) => {
    if (e.target !== vTrackRef.current) return;
    const el = scrollRef.current;
    const track = vTrackRef.current;
    if (!el || !track) return;

    const rect = track.getBoundingClientRect();
    const clickOffset = e.clientY - rect.top - vertical.size / 2;
    const topOffset = headerHeight > 0 ? headerHeight + 2 : 2;
    const trackHeight = el.clientHeight - topOffset - 2;
    const maxThumbTop = trackHeight - vertical.size;
    const maxScrollTop = el.scrollHeight - el.clientHeight;

    if (maxThumbTop > 0) {
      el.scrollTop = (clickOffset / maxThumbTop) * maxScrollTop;
    }
  };

  const handleHorizontalTrackPointerDown = (e: ReactPointerEvent) => {
    if (e.target !== hTrackRef.current) return;
    const el = scrollRef.current;
    const track = hTrackRef.current;
    if (!el || !track) return;

    const rect = track.getBoundingClientRect();
    const clickOffset = e.clientX - rect.left - horizontal.size / 2;
    const trackWidth = el.clientWidth - 4;
    const maxThumbLeft = trackWidth - horizontal.size;
    const maxScrollLeft = el.scrollWidth - el.clientWidth;

    if (maxThumbLeft > 0) {
      el.scrollLeft = (clickOffset / maxThumbLeft) * maxScrollLeft;
    }
  };

  const showVertical =
    vertical.visible &&
    (isHovered || isScrolling || isDraggingVertical || isDraggingHorizontal);
  const showHorizontal =
    horizontal.visible &&
    (isHovered || isScrolling || isDraggingVertical || isDraggingHorizontal);

  return (
    <>
      {/* Horizontal Bar */}
      <div
        ref={hTrackRef}
        className={cn(
          "el-scrollbar__bar is-horizontal",
          showHorizontal && "is-visible",
          isDraggingHorizontal && "is-active",
          className,
        )}
        style={{
          display: horizontal.visible ? "block" : "none",
        }}
        onPointerDown={handleHorizontalTrackPointerDown}
      >
        <div
          className={cn(
            "el-scrollbar__thumb",
            isDraggingHorizontal && "is-dragging",
          )}
          style={{
            width: `${horizontal.size}px`,
            transform: `translateX(${horizontal.offset}px)`,
          }}
          onPointerDown={handleHorizontalThumbPointerDown}
        />
      </div>

      {/* Vertical Bar (starts below header row, ends at bottom of table body) */}
      <div
        ref={vTrackRef}
        className={cn(
          "el-scrollbar__bar is-vertical",
          showVertical && "is-visible",
          isDraggingVertical && "is-active",
          className,
        )}
        style={{
          display: vertical.visible ? "block" : "none",
          top: headerHeight > 0 ? `${headerHeight + 2}px` : "2px",
          bottom: "2px",
        }}
        onPointerDown={handleVerticalTrackPointerDown}
      >
        <div
          className={cn(
            "el-scrollbar__thumb",
            isDraggingVertical && "is-dragging",
          )}
          style={{
            height: `${vertical.size}px`,
            transform: `translateY(${vertical.offset}px)`,
          }}
          onPointerDown={handleVerticalThumbPointerDown}
        />
      </div>
    </>
  );
}
