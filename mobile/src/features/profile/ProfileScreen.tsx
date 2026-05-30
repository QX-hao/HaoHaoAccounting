import { Text, TouchableOpacity, View } from 'react-native';
import { styles } from '../../theme/styles';

type Props = {
  onCreateCategory: () => void;
  onCreateAccount: () => void;
  onLogout: () => void;
};

export function ProfileScreen({ onCreateCategory, onCreateAccount, onLogout }: Props) {
  return (
    <View style={styles.card}>
      <Text style={styles.sectionTitle}>我的</Text>
      <TouchableOpacity style={styles.secondaryBtn} onPress={onCreateCategory}>
        <Text style={styles.secondaryBtnText}>新增自定义分类</Text>
      </TouchableOpacity>
      <TouchableOpacity style={styles.secondaryBtn} onPress={onCreateAccount}>
        <Text style={styles.secondaryBtnText}>新增自定义账户</Text>
      </TouchableOpacity>
      <TouchableOpacity style={[styles.secondaryBtn, { backgroundColor: '#fee2e2' }]} onPress={onLogout}>
        <Text style={{ color: '#991b1b', fontWeight: '600' }}>退出登录</Text>
      </TouchableOpacity>
    </View>
  );
}
