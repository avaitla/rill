import { BaseCanvasComponent } from "@rilldata/web-common/features/canvas/components/BaseCanvasComponent";
import type { AllKeys, InputParams } from "@rilldata/web-common/features/canvas/inspector/types";
import type { V1Resource } from "@rilldata/web-common/runtime-client";
import type { CanvasEntity, ComponentPath } from "../../stores/canvas-entity";
import {
  type CanvasComponentType,
  type SqlTableSpec,
} from "../types";
import SqlTable from "./SqlTable.svelte";

export { default as SqlTable } from "./SqlTable.svelte";

// Preset Vega-Lite specs
const VEGA_PRESETS: Record<string, object> = {
  line: {
    "$schema": "https://vega.github.io/schema/vega-lite/v5.json",
    data: { name: "table" },
    mark: "line",
    encoding: {
      x: { field: "date", type: "temporal", title: "Date" },
      y: { field: "value", type: "quantitative", title: "Value" },
    },
  },
  stacked_line: {
    "$schema": "https://vega.github.io/schema/vega-lite/v5.json",
    data: { name: "table" },
    mark: "line",
    encoding: {
      x: { field: "date", type: "temporal", title: "Date" },
      y: { field: "value", type: "quantitative", title: "Value" },
      color: { field: "category", type: "nominal", title: "Category" },
    },
  },
  bar: {
    "$schema": "https://vega.github.io/schema/vega-lite/v5.json",
    data: { name: "table" },
    mark: "bar",
    encoding: {
      x: { field: "category", type: "nominal", title: "Category" },
      y: { field: "value", type: "quantitative", title: "Value" },
    },
  },
  pie: {
    "$schema": "https://vega.github.io/schema/vega-lite/v5.json",
    data: { name: "table" },
    mark: { type: "arc", innerRadius: 0 },
    encoding: {
      theta: { field: "value", type: "quantitative" },
      color: { field: "category", type: "nominal", title: "Category" },
    },
  },
  single_number: {
    "$schema": "https://vega.github.io/schema/vega-lite/v5.json",
    data: { name: "table" },
    mark: { type: "text", fontSize: 64, fontWeight: "bold" },
    encoding: {
      text: { field: "value", type: "quantitative", format: ",.0f" },
    },
  },
};

export class SqlTableComponent extends BaseCanvasComponent<SqlTableSpec> {
  minSize = { width: 4, height: 4 };
  defaultSize = { width: 6, height: 8 };
  resetParams: string[] = [];
  type: CanvasComponentType = "sql_table";
  component = SqlTable;

  constructor(resource: V1Resource, parent: CanvasEntity, path: ComponentPath) {
    const defaultSpec: SqlTableSpec = {
      title: "",
      description: "",
      connector: "",
      sql: "",
      limit: 1000,
    };
    super(resource, parent, path, defaultSpec);
  }

  // Override updateProperty to handle preset-to-spec relationship
  updateProperty(key: AllKeys<SqlTableSpec>, value: SqlTableSpec[AllKeys<SqlTableSpec>]) {
    // When vega_preset changes to a valid preset, also update vega_lite_spec
    if (key === "vega_preset" && value && typeof value === "string" && VEGA_PRESETS[value]) {
      const presetSpec = VEGA_PRESETS[value];
      // First update the preset
      super.updateProperty(key, value);
      // Then update the spec with the preset's JSON
      super.updateProperty("vega_lite_spec" as AllKeys<SqlTableSpec>, JSON.stringify(presetSpec, null, 2) as SqlTableSpec[AllKeys<SqlTableSpec>]);
      return;
    }
    // For all other cases, use default behavior
    super.updateProperty(key, value);
  }

  isValid(spec: SqlTableSpec): boolean {
    return (
      typeof spec.connector === "string" &&
      spec.connector.trim().length > 0 &&
      typeof spec.sql === "string" &&
      spec.sql.trim().length > 0
    );
  }

  inputParams(): InputParams<SqlTableSpec> {
    return {
      options: {
        connector: {
          type: "text",
          label: "Connector",
          description: "Name of the connector to query (e.g., mysql_source)",
          meta: { placeholder: "mysql_source" },
        },
        sql: {
          type: "textArea",
          label: "SQL Query",
          description: "SQL query to execute against the connector",
          meta: { placeholder: "SELECT * FROM table LIMIT 100" },
        },
        display_type: {
          type: "select",
          label: "Display Type",
          optional: true,
          description: "How to display the query results",
          meta: {
            options: [
              { value: "table", label: "Table" },
              { value: "vega_lite", label: "Vega-Lite Chart" },
            ],
            default: "table",
          },
        },
        vega_preset: {
          type: "select",
          label: "Chart Preset",
          optional: true,
          description: "Select a preset chart type (populates Vega-Lite Spec)",
          meta: {
            options: [
              { value: "", label: "Custom" },
              { value: "line", label: "Line Chart" },
              { value: "stacked_line", label: "Stacked Line Chart" },
              { value: "bar", label: "Bar Chart" },
              { value: "pie", label: "Pie Chart" },
              { value: "single_number", label: "Single Number" },
            ],
            default: "",
          },
        },
        vega_lite_spec: {
          type: "textArea",
          label: "Vega-Lite Spec",
          optional: true,
          showInUI: true,
          description: "Vega-Lite JSON spec. Data source is named 'table'.",
          meta: {
            placeholder: '{"mark":"bar","encoding":{"x":{"field":"name"},"y":{"field":"value"}}}',
            rows: 8,
          },
        },
      },
      filter: {},
    };
  }

  static newComponentSpec(): SqlTableSpec {
    return {
      connector: "",
      sql: "SELECT 1 as example",
      limit: 1000,
      display_type: "table",
    };
  }
}
