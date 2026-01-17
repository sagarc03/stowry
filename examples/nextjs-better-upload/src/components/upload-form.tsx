"use client";

import { useUploadFiles } from "@better-upload/client";
import { useState, useCallback } from "react";
import { addUploadedFile } from "@/app/actions";

export function UploadForm() {
  const [dragActive, setDragActive] = useState(false);

  const { control, progresses } = useUploadFiles({
    route: "files",
    onUploadComplete: async ({ files }) => {
      for (const file of files) {
        await addUploadedFile({
          name: file.name,
          key: file.objectInfo.key,
          size: file.size,
          type: file.type,
        });
      }
    },
    onError: (error) => {
      console.error("Upload error:", error);
    },
  });

  const handleDrag = useCallback((e: React.DragEvent) => {
    e.preventDefault();
    e.stopPropagation();
    if (e.type === "dragenter" || e.type === "dragover") {
      setDragActive(true);
    } else if (e.type === "dragleave") {
      setDragActive(false);
    }
  }, []);

  const handleDrop = useCallback(
    (e: React.DragEvent) => {
      e.preventDefault();
      e.stopPropagation();
      setDragActive(false);
      if (e.dataTransfer.files && e.dataTransfer.files.length > 0) {
        control.upload(Array.from(e.dataTransfer.files));
      }
    },
    [control]
  );

  const handleChange = useCallback(
    (e: React.ChangeEvent<HTMLInputElement>) => {
      if (e.target.files && e.target.files.length > 0) {
        control.upload(Array.from(e.target.files));
      }
    },
    [control]
  );

  return (
    <div className="w-full space-y-6">
      <div
        className={`w-full rounded-xl border-2 border-dashed p-12 text-center transition-colors ${
          dragActive
            ? "border-blue-500 bg-blue-50 dark:bg-blue-950"
            : "border-zinc-300 dark:border-zinc-700"
        }`}
        onDragEnter={handleDrag}
        onDragLeave={handleDrag}
        onDragOver={handleDrag}
        onDrop={handleDrop}
      >
        <input
          type="file"
          multiple
          onChange={handleChange}
          className="hidden"
          id="file-upload"
          accept="image/*,application/pdf,text/*"
        />
        <label
          htmlFor="file-upload"
          className="cursor-pointer text-zinc-600 dark:text-zinc-400"
        >
          <div className="mb-4 text-4xl">📁</div>
          <p className="text-lg font-medium">
            Drag and drop files here, or{" "}
            <span className="text-blue-600 underline dark:text-blue-400">
              browse
            </span>
          </p>
          <p className="mt-2 text-sm text-zinc-500">
            Supports images, PDFs, and text files (max 10MB)
          </p>
        </label>
      </div>

      {progresses.length > 0 && (
        <div className="w-full space-y-3">
          <h2 className="text-lg font-semibold text-zinc-900 dark:text-zinc-100">
            Uploading
          </h2>
          {progresses.map((file) => (
            <div
              key={file.name}
              className="flex items-center justify-between rounded-lg border border-zinc-200 bg-white p-4 dark:border-zinc-800 dark:bg-zinc-900"
            >
              <div className="flex-1 min-w-0">
                <p className="truncate font-medium text-zinc-900 dark:text-zinc-100">
                  {file.name}
                </p>
                <p className="text-sm text-zinc-500">
                  {(file.size / 1024).toFixed(1)} KB
                </p>
              </div>
              <div className="ml-4">
                {file.status === "pending" && (
                  <span className="text-zinc-500">Waiting...</span>
                )}
                {file.status === "uploading" && (
                  <div className="flex items-center gap-2">
                    <div className="h-2 w-24 overflow-hidden rounded-full bg-zinc-200 dark:bg-zinc-700">
                      <div
                        className="h-full bg-blue-500 transition-all"
                        style={{ width: `${Math.round(file.progress * 100)}%` }}
                      />
                    </div>
                    <span className="text-sm text-zinc-600 dark:text-zinc-400">
                      {Math.round(file.progress * 100)}%
                    </span>
                  </div>
                )}
                {file.status === "complete" && (
                  <span className="text-green-600 dark:text-green-400">
                    Uploaded
                  </span>
                )}
                {file.status === "failed" && (
                  <span className="text-red-600 dark:text-red-400">Failed</span>
                )}
              </div>
            </div>
          ))}
        </div>
      )}
    </div>
  );
}
