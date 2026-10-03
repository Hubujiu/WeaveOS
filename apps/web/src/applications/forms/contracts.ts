/**
 * Form structure wire shapes from the accepted V030-013 ADR and contract
 * 269c77407375e8ff983861484a8548d03ba42a5a.
 * Type declarations only: the BFF remains the authority for validation,
 * normalization, authorization, and persistence.
 */
export type UUID = string;

export type FieldKind =
  | 'text' | 'multiline' | 'number' | 'money' | 'date' | 'datetime'
  | 'single_select' | 'multi_select' | 'boolean' | 'member' | 'department';

export type RoundingMode =
  | 'HALF_UP' | 'HALF_EVEN' | 'TOWARD_ZERO' | 'FLOOR' | 'CEILING';

/** Input keys may be omitted; explicit nulls are invalid. */
export type DecimalConfigInput = {
  precision?: number;
  scale?: number;
  roundingPlaces?: number;
  roundingMode?: RoundingMode;
};

export type DecimalConfig = {
  precision: number;
  scale: number;
  roundingPlaces: number;
  roundingMode: RoundingMode;
};

export type Option = { id: UUID; label: string };
export type PresentationInput = {
  helpText: string | null;
  displayTimeZone?: string | null;
};
export type Presentation = {
  helpText: string | null;
  displayTimeZone: string | null;
};

type FieldShape<K extends FieldKind, C, V, P> = {
  id: UUID;
  name: string;
  kind: K;
  required: boolean;
  default: V | null;
  config: C;
  presentation: P;
};

type StringField<K extends 'text' | 'multiline', P> =
  FieldShape<K, { maxLength: number | null }, string, P>;
type DecimalField<K extends 'number' | 'money', C, P> =
  FieldShape<K, C, string, P>;
type DateField<K extends 'date' | 'datetime', C, P> =
  FieldShape<K, C, string, P>;
type OptionField<K extends 'single_select' | 'multi_select', V, P> =
  FieldShape<K, { options: Option[] }, V, P>;
type ReferenceField<K extends 'member' | 'department', P> =
  FieldShape<K, Record<string, never>, UUID, P>;

export type FieldInput =
  | StringField<'text', PresentationInput>
  | StringField<'multiline', PresentationInput>
  | DecimalField<'number', DecimalConfigInput, PresentationInput>
  | DecimalField<'money', DecimalConfigInput, PresentationInput>
  | DateField<'date', Record<string, never>, PresentationInput>
  | DateField<'datetime', { precision?: 'minute' | 'second' | 'millisecond' }, PresentationInput>
  | OptionField<'single_select', UUID, PresentationInput>
  | OptionField<'multi_select', UUID[], PresentationInput>
  | FieldShape<'boolean', Record<string, never>, boolean, PresentationInput>
  | ReferenceField<'member', PresentationInput>
  | ReferenceField<'department', PresentationInput>;

export type Field =
  | StringField<'text', Presentation>
  | StringField<'multiline', Presentation>
  | DecimalField<'number', DecimalConfig, Presentation>
  | DecimalField<'money', DecimalConfig, Presentation>
  | DateField<'date', Record<string, never>, Presentation>
  | DateField<'datetime', { precision: 'minute' | 'second' | 'millisecond' }, Presentation>
  | OptionField<'single_select', UUID, Presentation>
  | OptionField<'multi_select', UUID[], Presentation>
  | FieldShape<'boolean', Record<string, never>, boolean, Presentation>
  | ReferenceField<'member', Presentation>
  | ReferenceField<'department', Presentation>;

export type SystemFieldCode =
  | 'id' | 'createdBy' | 'createdAt' | 'updatedAt' | 'recordVersion';
export type SystemField = {
  id: SystemFieldCode;
  kind: string;
  readOnly: true;
};

export type LayoutNodeInput =
  | { id: UUID; kind: 'field'; fieldId: UUID; span?: number }
  | { id: UUID; kind: 'system_field'; fieldId: SystemFieldCode; span?: number }
  | { id: UUID; kind: 'group'; title: string; children: LayoutNodeInput[]; span?: number }
  | { id: UUID; kind: 'divider' }
  | { id: UUID; kind: 'description'; text: string };

export type LayoutNode =
  | { id: UUID; kind: 'field'; fieldId: UUID; span: number }
  | { id: UUID; kind: 'system_field'; fieldId: SystemFieldCode; span: number }
  | { id: UUID; kind: 'group'; title: string; children: LayoutNode[]; span: number }
  | { id: UUID; kind: 'divider' }
  | { id: UUID; kind: 'description'; text: string };

export type Directory = {
  id: UUID; appId: UUID; name: string; parentId: UUID | null; position: number;
};
export type LogicalTable = {
  id: UUID; appId: UUID; name: string; directoryId: UUID | null;
  position: number; schemaVersion: number; schemaReady: boolean;
};
export type Form = {
  id: UUID; appId: UUID; tableId: UUID; name: string;
  directoryId: UUID | null; position: number; viewVersion: number;
};
export type Structure = {
  appId: UUID; structureVersion: number;
  directories: Directory[]; tables: LogicalTable[]; forms: Form[];
  capabilities: { canManageDefinition: boolean };
};
export type DirectoryResult = { directory: Directory; structureVersion: number };
export type TableResult = { table: LogicalTable; structureVersion: number };
export type FormSource =
  | { kind: 'new_table' }
  | { kind: 'existing_table'; tableId: UUID };
export type FormResult = {
  table: LogicalTable; form: Form; structureVersion: number;
};
export type Definition = {
  appId: UUID; table: LogicalTable; form: Form;
  fields: Field[]; systemFields: SystemField[]; layout: LayoutNode[];
  capabilities: { canManageDefinition: boolean };
};
export type OptionMapping = {
  fieldId: UUID; fromOptionId: UUID; toOptionId: UUID | null;
};
export type DefinitionInput = {
  expectedSchemaVersion: number; expectedViewVersion: number;
  fields: FieldInput[]; layout: LayoutNodeInput[]; optionMappings: OptionMapping[];
};
export type SchemaChange = {
  kind: 'add' | 'remove' | 'change_type' | 'change_config'
    | 'change_default' | 'change_required';
  fieldId: UUID; beforeKind: FieldKind | null; afterKind: FieldKind | null;
};
export type ChangePlan = {
  schemaChanges: SchemaChange[]; metadataChanged: boolean; layoutChanged: boolean;
};
export type Impact = {
  fieldId: UUID; kind: 'column_removal' | 'option_mapping';
  nonNullRows: number; optionId: UUID | null;
};
export type Dependency = {
  fieldId: UUID; kind: 'enabled_flow' | 'in_flight' | 'view_layout';
  resourceId: UUID;
};
export type Preflight = {
  appId: UUID; tableId: UUID; viewId: UUID;
  schemaVersion: number; viewVersion: number;
  dataRevision: number; dependencyRevision: number;
  plan: ChangePlan; impacts: Impact[]; dependencies: Dependency[];
  blockingIssues: { code: string; fieldIds: UUID[] }[];
  saveAllowed: boolean;
  confirmation: { token: string; expiresAt: string } | null;
};
export type DefinitionSave = {
  operationId: UUID; definition: Definition; appliedPlan: ChangePlan;
};
