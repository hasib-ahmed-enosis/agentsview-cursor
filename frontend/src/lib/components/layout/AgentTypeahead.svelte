<script lang="ts">
  import { Typeahead } from "@kenn-io/kit-ui";
  import { m } from "../../i18n/index.js";
  import type { DbAgentInfo as AgentInfo } from "../../api/generated/index.js";
  import { agentLabel } from "../../utils/agents.js";

  interface Props {
    agents: AgentInfo[];
    value: string;
    onselect: (value: string) => void;
    onquery?: (query: string) => void;
    includeAll?: boolean;
    placeholder?: string;
    title?: string;
    emptyLabel?: string;
    disabled?: boolean;
  }

  let {
    agents,
    value,
    onselect,
    onquery = undefined,
    includeAll = true,
    placeholder = m.shared_agent_filter_placeholder(),
    title = m.shared_select_agent(),
    emptyLabel = m.shared_no_matching_agents(),
    disabled = false,
  }: Props = $props();

  const allOption = {
    name: "",
    label: m.shared_all_agents(),
    displayLabel: m.shared_all_agents(),
    count: 0,
  };

  const options = $derived.by(() => {
    const items = agents.map((a) => ({
      name: a.name,
      label: `${agentLabel(a.name)} (${a.session_count})`,
      displayLabel: agentLabel(a.name),
      count: a.session_count,
    }));
    return includeAll ? [allOption, ...items] : items;
  });

  const displayValue = $derived(
    value
      ? (agents.find((a) => a.name === value)
          ? agentLabel(value)
          : value)
      : includeAll
        ? m.shared_all_agents()
        : placeholder,
  );
</script>

<Typeahead
  {disabled}
  {options}
  {value}
  fallbackLabel={displayValue}
  {placeholder}
  inputAttributes={{ "data-1p-ignore": "true" }}
  {title}
  {emptyLabel}
  {onquery}
  {onselect}
/>
