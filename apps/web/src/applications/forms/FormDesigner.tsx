/** Minimal non-behavioral test target; behavior follows an observed RED. */
export type FormDesignerProps = {
  appId: string;
  viewId: string;
  onDirtyChange?: (dirty: boolean) => void;
  onBack?: () => void;
};

export function FormDesigner(_props: FormDesignerProps) {
  return null;
}
