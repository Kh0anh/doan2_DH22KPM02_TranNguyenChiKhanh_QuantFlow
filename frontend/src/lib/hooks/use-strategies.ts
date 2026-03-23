// ===================================================================
// QuantFlow — useStrategies Hook
// Task 3.2.6 — Strategy List Page (CRUD + pagination + search)
// ===================================================================
//
// Responsibilities:
//   - Fetch paginated strategy list via GET /strategies
//   - Server-side search (ILIKE) with client debounce 300ms
//   - Delete strategy with 409 STRATEGY_IN_USE handling
//   - Import strategy from JSON file
//   - Export strategy as .json download
//
// Usage:
//   const { strategies, pagination, isLoading, ... } = useStrategies();
// ===================================================================

"use client";

import { useCallback, useEffect, useRef, useState } from "react";
import { strategyApi } from "@/lib/api-client";
import { ApiError } from "@/lib/api-client";
import { toast } from "sonner";

// -----------------------------------------------------------------
// Types
// -----------------------------------------------------------------

export interface StrategyListItem {
  id: string;
  name: string;
  version: number;
  status: string;
  createdAt: string;
  updatedAt: string;
}

export interface PaginationInfo {
  page: number;
  limit: number;
  total: number;
  totalPages: number;
}

// -----------------------------------------------------------------
// Hook
// -----------------------------------------------------------------

const PAGE_SIZE = 10;

export function useStrategies() {
  const [strategies, setStrategies] = useState<StrategyListItem[]>([]);
  const [pagination, setPagination] = useState<PaginationInfo>({
    page: 1,
    limit: PAGE_SIZE,
    total: 0,
    totalPages: 0,
  });
  const [search, setSearch] = useState("");
  const [debouncedSearch, setDebouncedSearch] = useState("");
  const [isLoading, setIsLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);

  // Debounce search input (300ms)
  const debounceRef = useRef<ReturnType<typeof setTimeout> | null>(null);

  const handleSearchChange = useCallback((value: string) => {
    setSearch(value);
    if (debounceRef.current) clearTimeout(debounceRef.current);
    debounceRef.current = setTimeout(() => {
      setDebouncedSearch(value);
    }, 300);
  }, []);

  // Cleanup debounce on unmount
  useEffect(() => {
    return () => {
      if (debounceRef.current) clearTimeout(debounceRef.current);
    };
  }, []);

  // ---------------------------------------------------------------
  // Fetch strategies
  // ---------------------------------------------------------------

  const fetchStrategies = useCallback(
    async (page: number, searchQuery: string) => {
      setIsLoading(true);
      setError(null);
      try {
        const res = await strategyApi.list({
          page,
          limit: PAGE_SIZE,
          search: searchQuery || undefined,
        });

        // Map snake_case API response → camelCase frontend model
        setStrategies(
          res.data.map((s) => ({
            id: s.id,
            name: s.name,
            version: s.version,
            status: s.status,
            createdAt: s.created_at,
            updatedAt: s.updated_at,
          })),
        );
        setPagination({
          page: res.pagination.page,
          limit: res.pagination.limit,
          total: res.pagination.total,
          totalPages: res.pagination.total_pages,
        });
      } catch (err) {
        const message =
          err instanceof ApiError
            ? err.message
            : "Không thể tải danh sách chiến lược.";
        setError(message);
      } finally {
        setIsLoading(false);
      }
    },
    [],
  );

  // Refetch when page or debounced search changes
  useEffect(() => {
    void fetchStrategies(pagination.page, debouncedSearch);
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [pagination.page, debouncedSearch, fetchStrategies]);

  // Reset to page 1 when search changes
  useEffect(() => {
    setPagination((prev) => (prev.page !== 1 ? { ...prev, page: 1 } : prev));
  }, [debouncedSearch]);

  // ---------------------------------------------------------------
  // Page change
  // ---------------------------------------------------------------

  const goToPage = useCallback((page: number) => {
    setPagination((prev) => ({ ...prev, page }));
  }, []);

  // ---------------------------------------------------------------
  // Delete strategy
  // ---------------------------------------------------------------

  const deleteStrategy = useCallback(
    async (id: string) => {
      try {
        await strategyApi.delete(id);
        toast.success("Đã xóa chiến lược thành công.");
        // Re-fetch current page
        void fetchStrategies(pagination.page, debouncedSearch);
      } catch (err) {
        if (err instanceof ApiError && err.status === 409) {
          toast.error(
            err.message ||
              "Chiến lược đang được sử dụng bởi Bot đang chạy. Vui lòng dừng Bot trước.",
          );
        } else {
          toast.error("Xóa chiến lược thất bại. Vui lòng thử lại.");
        }
        throw err; // Re-throw so caller (dialog) can handle
      }
    },
    [fetchStrategies, pagination.page, debouncedSearch],
  );

  // ---------------------------------------------------------------
  // Import strategy from JSON file
  // ---------------------------------------------------------------

  const importStrategy = useCallback(
    async (file: File) => {
      try {
        const text = await file.text();
        const json = JSON.parse(text) as Record<string, unknown>;

        let name: string;
        let logicJson: Record<string, unknown>;

        if (json.name && json.logic_json) {
          // Standard format: { name, logic_json, ... }
          name = json.name as string;
          logicJson = json.logic_json as Record<string, unknown>;
        } else if (json.blocks) {
          // Legacy/raw Blockly state: { blocks: { ... } }
          // Use filename (without extension) as strategy name
          name = file.name.replace(/\.json$/i, "").replace(/[-_]+/g, " ");
          logicJson = json;
        } else {
          toast.error(
            "File JSON không hợp lệ. Cần có trường 'name' và 'logic_json', hoặc là file Blockly hợp lệ.",
          );
          return;
        }

        await strategyApi.importStrategy({
          name,
          logic_json: logicJson,
        });

        toast.success(`Nhập chiến lược "${name}" thành công.`);
        // Re-fetch to show new strategy
        void fetchStrategies(1, debouncedSearch);
      } catch (err) {
        if (err instanceof SyntaxError) {
          toast.error("File không phải JSON hợp lệ.");
        } else if (err instanceof ApiError) {
          toast.error(err.message || "Nhập chiến lược thất bại.");
        } else {
          toast.error("Nhập chiến lược thất bại. Vui lòng thử lại.");
        }
      }
    },
    [fetchStrategies, debouncedSearch],
  );

  // ---------------------------------------------------------------
  // Export strategy as JSON
  // ---------------------------------------------------------------

  const exportStrategy = useCallback(async (id: string, name: string) => {
    try {
      await strategyApi.exportStrategy(id, name);
      toast.success("Xuất chiến lược thành công.");
    } catch {
      toast.error("Xuất chiến lược thất bại. Vui lòng thử lại.");
    }
  }, []);

  // ---------------------------------------------------------------
  // Refresh (manual re-fetch)
  // ---------------------------------------------------------------

  const refresh = useCallback(() => {
    void fetchStrategies(pagination.page, debouncedSearch);
  }, [fetchStrategies, pagination.page, debouncedSearch]);

  return {
    // Data
    strategies,
    pagination,
    search,
    isLoading,
    error,
    // Actions
    handleSearchChange,
    goToPage,
    deleteStrategy,
    importStrategy,
    exportStrategy,
    refresh,
  };
}
