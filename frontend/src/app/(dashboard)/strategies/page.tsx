// ===================================================================
// QuantFlow — Strategy List Page
// Task 3.2.6 — CRUD + pagination + search + Open-in-Editor
// ===================================================================
//
// Displays all strategies in a searchable, paginated table.
// Actions per row: Open in Editor, Export JSON, Delete.
// Header actions: Import from JSON file, Create new strategy.
//
// Integration:
//   - useStrategies hook for data & actions
//   - useEditorStore.openTab() + router.push("/editor") for "Open in Editor"
//   - useEditorStore.openNewTab() for "Create New Strategy"
// ===================================================================

"use client";

import { useRef, useState } from "react";
import { useRouter } from "next/navigation";
import {
  Search,
  Plus,
  Upload,
  MoreHorizontal,
  ExternalLink,
  Download,
  Trash2,
  ChevronLeft,
  ChevronRight,
  AlertTriangle,
  Loader2,
  FileCode2,
} from "lucide-react";

import { useStrategies } from "@/lib/hooks/use-strategies";
import { useEditorStore } from "@/store/editor-store";
import { formatDateTime } from "@/lib/utils";

import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Badge } from "@/components/ui/badge";
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from "@/components/ui/table";
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuSeparator,
  DropdownMenuTrigger,
} from "@/components/ui/dropdown-menu";
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog";

// -----------------------------------------------------------------
// Status badge helper
// -----------------------------------------------------------------

function StatusBadge({ status }: { status: string }) {
  const lower = status.toLowerCase();
  const variant =
    lower === "valid"
      ? "default"
      : lower === "draft"
        ? "secondary"
        : "outline";

  const label =
    lower === "valid"
      ? "Hợp lệ"
      : lower === "draft"
        ? "Nháp"
        : lower === "archived"
          ? "Đã lưu trữ"
          : status;

  return <Badge variant={variant}>{label}</Badge>;
}

// -----------------------------------------------------------------
// Main Page Component
// -----------------------------------------------------------------

export default function StrategiesPage() {
  const router = useRouter();
  const fileInputRef = useRef<HTMLInputElement>(null);
  const openNewTab = useEditorStore((s) => s.openNewTab);
  const openTab = useEditorStore((s) => s.openTab);

  // Strategy list state
  const {
    strategies,
    pagination,
    search,
    isLoading,
    error,
    handleSearchChange,
    goToPage,
    deleteStrategy,
    importStrategy,
    exportStrategy,
  } = useStrategies();

  // Delete confirmation dialog
  const [deleteTarget, setDeleteTarget] = useState<{
    id: string;
    name: string;
  } | null>(null);
  const [isDeleting, setIsDeleting] = useState(false);

  // ---------------------------------------------------------------
  // Handlers
  // ---------------------------------------------------------------

  /** Open strategy in multi-tab editor */
  const handleOpenInEditor = (strategyId: string, name: string) => {
    openTab(strategyId, name);
    router.push("/editor");
  };

  /** Create new strategy in editor */
  const handleCreateNew = () => {
    openNewTab();
    router.push("/editor");
  };

  /** Trigger file picker for import */
  const handleImportClick = () => {
    fileInputRef.current?.click();
  };

  /** Handle file selected for import */
  const handleFileSelected = async (
    e: React.ChangeEvent<HTMLInputElement>,
  ) => {
    const file = e.target.files?.[0];
    if (!file) return;
    await importStrategy(file);
    // Reset input so same file can be re-selected
    if (fileInputRef.current) fileInputRef.current.value = "";
  };

  /** Confirm and execute delete */
  const handleConfirmDelete = async () => {
    if (!deleteTarget) return;
    setIsDeleting(true);
    try {
      await deleteStrategy(deleteTarget.id);
      setDeleteTarget(null);
    } catch {
      // Error toast is shown by the hook
    } finally {
      setIsDeleting(false);
    }
  };

  // ---------------------------------------------------------------
  // Render
  // ---------------------------------------------------------------

  return (
    <div className="flex flex-col gap-6 p-6">
      {/* Page Header */}
      <div className="flex items-center justify-between">
        <div>
          <h1 className="text-2xl font-bold tracking-tight">
            Chiến lược giao dịch
          </h1>
          <p className="text-sm text-muted-foreground mt-1">
            Quản lý và tổ chức các mẫu chiến lược của bạn.
          </p>
        </div>
        <div className="flex items-center gap-2">
          {/* Hidden file input for import */}
          <input
            ref={fileInputRef}
            type="file"
            accept=".json"
            className="hidden"
            onChange={handleFileSelected}
          />
          <Button variant="outline" size="sm" onClick={handleImportClick}>
            <Upload className="h-4 w-4 mr-2" />
            Nhập từ file
          </Button>
          <Button size="sm" onClick={handleCreateNew}>
            <Plus className="h-4 w-4 mr-2" />
            Tạo mới
          </Button>
        </div>
      </div>

      {/* Search Bar */}
      <div className="relative max-w-sm">
        <Search className="absolute left-3 top-1/2 -translate-y-1/2 h-4 w-4 text-muted-foreground" />
        <Input
          placeholder="Tìm kiếm chiến lược..."
          value={search}
          onChange={(e) => handleSearchChange(e.target.value)}
          className="pl-9"
        />
      </div>

      {/* Content Area */}
      {error ? (
        /* Error State */
        <div className="flex flex-col items-center justify-center py-16 gap-3">
          <AlertTriangle className="h-10 w-10 text-danger" />
          <p className="text-sm text-danger">{error}</p>
          <Button variant="outline" size="sm" onClick={() => goToPage(1)}>
            Thử lại
          </Button>
        </div>
      ) : isLoading ? (
        /* Loading State */
        <div className="flex items-center justify-center py-16">
          <Loader2 className="h-6 w-6 animate-spin text-muted-foreground" />
          <span className="ml-2 text-sm text-muted-foreground">
            Đang tải danh sách...
          </span>
        </div>
      ) : strategies.length === 0 ? (
        /* Empty State */
        <div className="flex flex-col items-center justify-center py-16 gap-4">
          <FileCode2 className="h-12 w-12 text-muted-foreground/50" />
          <div className="text-center">
            <p className="text-sm font-medium text-foreground">
              {search
                ? "Không tìm thấy chiến lược nào"
                : "Chưa có chiến lược nào"}
            </p>
            <p className="text-sm text-muted-foreground mt-1">
              {search
                ? `Không có kết quả cho "${search}".`
                : "Tạo chiến lược mới hoặc nhập từ file JSON."}
            </p>
          </div>
          {!search && (
            <Button size="sm" onClick={handleCreateNew}>
              <Plus className="h-4 w-4 mr-2" />
              Tạo chiến lược đầu tiên
            </Button>
          )}
        </div>
      ) : (
        /* Strategy Table */
        <>
          <div className="rounded-md border">
            <Table>
              <TableHeader>
                <TableRow>
                  <TableHead className="w-[35%]">Tên chiến lược</TableHead>
                  <TableHead className="w-[10%]">Phiên bản</TableHead>
                  <TableHead className="w-[12%]">Trạng thái</TableHead>
                  <TableHead className="w-[18%]">Ngày tạo</TableHead>
                  <TableHead className="w-[18%]">Cập nhật lần cuối</TableHead>
                  <TableHead className="w-[7%] text-right">
                    Hành động
                  </TableHead>
                </TableRow>
              </TableHeader>
              <TableBody>
                {strategies.map((strategy) => (
                  <TableRow
                    key={strategy.id}
                    className="cursor-pointer hover:bg-muted/50"
                    onDoubleClick={() =>
                      handleOpenInEditor(strategy.id, strategy.name)
                    }
                  >
                    <TableCell className="font-medium">
                      {strategy.name}
                    </TableCell>
                    <TableCell className="text-muted-foreground">
                      v{strategy.version}
                    </TableCell>
                    <TableCell>
                      <StatusBadge status={strategy.status} />
                    </TableCell>
                    <TableCell className="text-muted-foreground">
                      {formatDateTime(strategy.createdAt)}
                    </TableCell>
                    <TableCell className="text-muted-foreground">
                      {formatDateTime(strategy.updatedAt)}
                    </TableCell>
                    <TableCell className="text-right">
                      <DropdownMenu>
                        <DropdownMenuTrigger asChild>
                          <Button variant="ghost" size="icon" className="h-8 w-8">
                            <MoreHorizontal className="h-4 w-4" />
                            <span className="sr-only">Mở menu</span>
                          </Button>
                        </DropdownMenuTrigger>
                        <DropdownMenuContent align="end">
                          <DropdownMenuItem
                            onClick={() =>
                              handleOpenInEditor(strategy.id, strategy.name)
                            }
                          >
                            <ExternalLink className="h-4 w-4 mr-2" />
                            Mở trong Editor
                          </DropdownMenuItem>
                          <DropdownMenuItem
                            onClick={() =>
                              exportStrategy(strategy.id, strategy.name)
                            }
                          >
                            <Download className="h-4 w-4 mr-2" />
                            Xuất JSON
                          </DropdownMenuItem>
                          <DropdownMenuSeparator />
                          <DropdownMenuItem
                            className="text-destructive focus:text-destructive"
                            onClick={() =>
                              setDeleteTarget({
                                id: strategy.id,
                                name: strategy.name,
                              })
                            }
                          >
                            <Trash2 className="h-4 w-4 mr-2" />
                            Xóa
                          </DropdownMenuItem>
                        </DropdownMenuContent>
                      </DropdownMenu>
                    </TableCell>
                  </TableRow>
                ))}
              </TableBody>
            </Table>
          </div>

          {/* Pagination Controls */}
          {pagination.totalPages > 1 && (
            <div className="flex items-center justify-between">
              <p className="text-sm text-muted-foreground">
                Hiển thị{" "}
                {(pagination.page - 1) * pagination.limit + 1}–
                {Math.min(
                  pagination.page * pagination.limit,
                  pagination.total,
                )}{" "}
                / {pagination.total} chiến lược
              </p>
              <div className="flex items-center gap-2">
                <Button
                  variant="outline"
                  size="icon"
                  className="h-8 w-8"
                  disabled={pagination.page <= 1}
                  onClick={() => goToPage(pagination.page - 1)}
                >
                  <ChevronLeft className="h-4 w-4" />
                </Button>
                <span className="text-sm text-muted-foreground min-w-[80px] text-center">
                  Trang {pagination.page} / {pagination.totalPages}
                </span>
                <Button
                  variant="outline"
                  size="icon"
                  className="h-8 w-8"
                  disabled={pagination.page >= pagination.totalPages}
                  onClick={() => goToPage(pagination.page + 1)}
                >
                  <ChevronRight className="h-4 w-4" />
                </Button>
              </div>
            </div>
          )}
        </>
      )}

      {/* Delete Confirmation Dialog */}
      <Dialog
        open={!!deleteTarget}
        onOpenChange={(open) => !open && setDeleteTarget(null)}
      >
        <DialogContent>
          <DialogHeader>
            <DialogTitle>Xác nhận xóa chiến lược</DialogTitle>
            <DialogDescription>
              Bạn có chắc chắn muốn xóa chiến lược{" "}
              <strong>&quot;{deleteTarget?.name}&quot;</strong>? Hành động này
              không thể hoàn tác.
            </DialogDescription>
          </DialogHeader>
          <DialogFooter>
            <Button
              variant="outline"
              onClick={() => setDeleteTarget(null)}
              disabled={isDeleting}
            >
              Hủy
            </Button>
            <Button
              variant="destructive"
              onClick={handleConfirmDelete}
              disabled={isDeleting}
            >
              {isDeleting ? (
                <>
                  <Loader2 className="h-4 w-4 mr-2 animate-spin" />
                  Đang xóa...
                </>
              ) : (
                <>
                  <Trash2 className="h-4 w-4 mr-2" />
                  Xóa chiến lược
                </>
              )}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>
    </div>
  );
}
