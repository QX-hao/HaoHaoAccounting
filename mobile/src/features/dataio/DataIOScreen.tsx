import { Text, TextInput, TouchableOpacity, View } from 'react-native';
import type { ImportPreview } from '../../shared/types/accounting';
import { styles } from '../../theme/styles';

type Props = {
  csvText: string;
  exportText: string;
  selectedImportFile: string;
  preview: ImportPreview | null;
  onCSVTextChange: (value: string) => void;
  onPickFile: () => void;
  onPreview: () => void;
  onImport: () => void;
  onExport: () => void;
  onCopyExport: () => void;
  onShareExport: () => void;
};

export function DataIOScreen({
  csvText,
  exportText,
  selectedImportFile,
  preview,
  onCSVTextChange,
  onPickFile,
  onPreview,
  onImport,
  onExport,
  onCopyExport,
  onShareExport,
}: Props) {
  return (
    <View style={styles.card}>
      <Text style={styles.sectionTitle}>导入 CSV/XLSX</Text>
      <Text style={styles.muted}>CSV 字段：occurred_at,type,amount,category,account,note,tags,source</Text>
      <TouchableOpacity style={styles.secondaryBtn} onPress={onPickFile}>
        <Text style={styles.secondaryBtnText}>从系统文件选择 CSV/XLSX</Text>
      </TouchableOpacity>
      {selectedImportFile ? <Text style={styles.muted}>已选择：{selectedImportFile}</Text> : null}
      <TextInput
        style={[styles.input, { minHeight: 150 }]}
        multiline
        placeholder="选择 CSV 后会自动填入，也可以粘贴 CSV 内容；XLSX 会直接上传预览"
        value={csvText}
        onChangeText={onCSVTextChange}
      />
      <View style={styles.row}>
        <TouchableOpacity style={styles.secondaryBtnCompact} onPress={onPreview}>
          <Text style={styles.secondaryBtnText}>预览校验</Text>
        </TouchableOpacity>
        <TouchableOpacity style={styles.primaryBtnCompact} onPress={onImport}>
          <Text style={styles.primaryBtnText}>开始导入</Text>
        </TouchableOpacity>
      </View>
      {preview ? (
        <View style={styles.listItem}>
          <Text>总计 {preview.totalRows} 行 · 有效 {preview.validRows} · 重复 {preview.duplicateRows} · 失败 {preview.failedRows}</Text>
          {preview.rows.map((row) => (
            <Text key={row.line} style={row.valid && !row.duplicate ? styles.muted : styles.error}>
              {row.line}: {row.valid ? `${row.type} ${row.amount} ${row.category}${row.duplicate ? ` · ${row.duplicateReason || '重复风险'}` : ''}` : row.error}
            </Text>
          ))}
        </View>
      ) : null}

      <Text style={styles.sectionTitle}>导出 CSV</Text>
      <TouchableOpacity style={styles.secondaryBtn} onPress={onExport}>
        <Text style={styles.secondaryBtnText}>生成导出文本</Text>
      </TouchableOpacity>
      {exportText ? (
        <>
          <View style={styles.row}>
            <TouchableOpacity style={styles.secondaryBtnCompact} onPress={onCopyExport}>
              <Text style={styles.secondaryBtnText}>复制</Text>
            </TouchableOpacity>
            <TouchableOpacity style={styles.primaryBtnCompact} onPress={onShareExport}>
              <Text style={styles.primaryBtnText}>分享/保存 CSV</Text>
            </TouchableOpacity>
          </View>
          <TextInput style={[styles.input, { minHeight: 160 }]} multiline value={exportText} onChangeText={() => undefined} />
        </>
      ) : null}
    </View>
  );
}
