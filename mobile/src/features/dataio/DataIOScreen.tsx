import { Text, TextInput, TouchableOpacity, View } from 'react-native';
import type { ImportPreview } from '../../shared/types/accounting';
import { styles } from '../../theme/styles';

type Props = {
  csvText: string;
  exportText: string;
  preview: ImportPreview | null;
  onCSVTextChange: (value: string) => void;
  onPreview: () => void;
  onImport: () => void;
  onExport: () => void;
};

export function DataIOScreen({ csvText, exportText, preview, onCSVTextChange, onPreview, onImport, onExport }: Props) {
  return (
    <View style={styles.card}>
      <Text style={styles.sectionTitle}>导入 CSV</Text>
      <Text style={styles.muted}>字段：occurred_at,type,amount,category,account,note,tags,source</Text>
      <TextInput
        style={[styles.input, { minHeight: 150 }]}
        multiline
        placeholder="粘贴 CSV 内容"
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
          <Text>总计 {preview.totalRows} 行 · 有效 {preview.validRows} · 失败 {preview.failedRows}</Text>
          {preview.rows.map((row) => (
            <Text key={row.line} style={row.valid ? styles.muted : styles.error}>
              {row.line}: {row.valid ? `${row.type} ${row.amount} ${row.category}` : row.error}
            </Text>
          ))}
        </View>
      ) : null}

      <Text style={styles.sectionTitle}>导出 CSV</Text>
      <TouchableOpacity style={styles.secondaryBtn} onPress={onExport}>
        <Text style={styles.secondaryBtnText}>生成导出文本</Text>
      </TouchableOpacity>
      {exportText ? <TextInput style={[styles.input, { minHeight: 160 }]} multiline value={exportText} onChangeText={() => undefined} /> : null}
    </View>
  );
}
