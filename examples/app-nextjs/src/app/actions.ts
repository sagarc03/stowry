"use server";

import { saveFile } from "@/lib/db";
import { revalidatePath } from "next/cache";

export async function addUploadedFile(file: {
  name: string;
  key: string;
  size: number;
  type: string;
}) {
  saveFile({
    name: file.name,
    key: file.key,
    size: file.size,
    content_type: file.type,
  });
  revalidatePath("/");
}
