import { createContext, useCallback, useContext, useLayoutEffect, useMemo, useState, type ReactNode } from 'react';
import { createPortal } from 'react-dom';

type SlotName = 'palette' | 'properties' | 'context' | 'actions';
type Targets = Record<SlotName, HTMLDivElement | null>;
type DesignerSlotContext = {
 active: boolean;
 targets: Targets;
 refs: Record<SlotName, (node: HTMLDivElement | null) => void>;
 register: (scope: string) => () => void;
};
const Context = createContext<DesignerSlotContext | null>(null);

/** Only render targets and their scoped lease live here; form state stays in the designer. */
export function DesignerSlotsProvider({ children }: { children: ReactNode }) {
 const [targets, setTargets] = useState<Targets>({ palette: null, properties: null, context: null, actions: null });
 const [lease, setLease] = useState<{ scope: string; token: symbol } | null>(null);
 const refs = useMemo(() => Object.fromEntries(
  (['palette', 'properties', 'context', 'actions'] as const).map(name => [name, (node: HTMLDivElement | null) =>
   setTargets(current => current[name] === node ? current : { ...current, [name]: node })]),
 ) as DesignerSlotContext['refs'], []);
 const register = useCallback((scope: string) => {
  const token = Symbol(scope);
  setLease({ scope, token });
  return () => setLease(current => current?.token === token ? null : current);
 }, []);
 const value = useMemo(() => ({ active: !!lease, targets, refs, register }), [lease, targets, refs, register]);
 return <Context.Provider value={value}>{children}</Context.Provider>;
}

export const useDesignerSlots = () => useContext(Context);

export function DesignerSlots({ scope, palette, properties, context, actions }: {
 scope: string; palette: ReactNode; properties: ReactNode; context: ReactNode; actions: ReactNode;
}) {
 const slots = useDesignerSlots();
 const register = slots?.register;
 useLayoutEffect(() => register?.(scope), [register, scope]);
 if (!slots) return null;
 const content = { palette, properties, context, actions };
 return <>{(Object.keys(content) as SlotName[]).map(name => slots.targets[name]
  ? createPortal(content[name], slots.targets[name]!, name) : null)}</>;
}
