/** Minimal non-behavioral test target; behavior follows an observed RED. */
export type ApplicationStructureProps = {
  appId: string;
  onOpenForm?: (viewId: string) => void;
  onDirtyChange?: (dirty: boolean) => void;
};

export function ApplicationStructurePanel(_props: ApplicationStructureProps) {
  return null;
}
