export function formatNumber(value: number | undefined | null) {
  return new Intl.NumberFormat("en-US").format(value ?? 0);
}

export function formatTime(value: string | undefined | null) {
  if (!value) {
    return "-";
  }
  const date = new Date(value);
  if (Number.isNaN(date.getTime())) {
    return value;
  }
  return date.toISOString().slice(11, 23);
}

export function formatRelative(value: string | undefined | null) {
  if (!value) {
    return "-";
  }

  const date = new Date(value);
  const diffSeconds = Math.max(0, Math.round((Date.now() - date.getTime()) / 1000));

  if (Number.isNaN(date.getTime())) {
    return value;
  }
  if (diffSeconds < 2) {
    return "Just now";
  }
  if (diffSeconds < 60) {
    return `${diffSeconds}s ago`;
  }
  if (diffSeconds < 3600) {
    return `${Math.round(diffSeconds / 60)}m ago`;
  }
  return `${Math.round(diffSeconds / 3600)}h ago`;
}

export function truncateMiddle(value: string, max = 44) {
  if (!value || value.length <= max) {
    return value;
  }
  return `${value.slice(0, max - 3)}...`;
}
