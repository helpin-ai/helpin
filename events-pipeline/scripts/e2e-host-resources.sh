#!/usr/bin/env bash

parse_macos_available_memory_kib() {
  awk '
    /page size of/ {
      for (field = 1; field <= NF; field++) {
        if ($field == "of") {
          page_size = $(field + 1)
        }
      }
    }
    /^Pages free:/ { free_pages = $3; gsub(/\./, "", free_pages) }
    /^Pages inactive:/ { inactive_pages = $3; gsub(/\./, "", inactive_pages) }
    /^Pages speculative:/ { speculative_pages = $3; gsub(/\./, "", speculative_pages) }
    END {
      if (!page_size) exit 1
      printf "%.0f\n", (free_pages + inactive_pages + speculative_pages) * page_size / 1024
    }
  '
}

parse_macos_swap_free_kib() {
  awk '
    function kib(value, unit, amount) {
      unit = substr(value, length(value), 1)
      amount = substr(value, 1, length(value) - 1) + 0
      if (unit == "G") return amount * 1024 * 1024
      if (unit == "M") return amount * 1024
      if (unit == "K") return amount
      return value + 0
    }
    {
      for (field = 1; field <= NF; field++) {
        if ($field == "free" && $(field + 1) == "=") {
          printf "%.0f\n", kib($(field + 2))
          found = 1
          exit
        }
      }
    }
    END { if (!found) exit 1 }
  '
}

sample_host_memory_kib() {
  local available_kib
  local swap_free_kib

  if [[ -r /proc/meminfo ]]; then
    awk '
      /MemAvailable:/ { available = $2 }
      /SwapFree:/ { swap = $2 }
      END { printf "%s %s\n", available + 0, swap + 0 }
    ' /proc/meminfo
    return
  fi

  if command -v vm_stat >/dev/null 2>&1; then
    available_kib=$(vm_stat | parse_macos_available_memory_kib) || available_kib=0
    if command -v sysctl >/dev/null 2>&1; then
      swap_free_kib=$(sysctl -n vm.swapusage 2>/dev/null | parse_macos_swap_free_kib) || swap_free_kib=0
    else
      swap_free_kib=0
    fi
    printf '%s %s\n' "$available_kib" "$swap_free_kib"
    return
  fi

  echo '0 0'
}
