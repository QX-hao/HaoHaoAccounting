import { Text, TextInput, TouchableOpacity, View } from 'react-native';
import { StatusBar } from 'expo-status-bar';
import { styles } from '../../theme/styles';

type Props = {
  username: string;
  password: string;
  error: string;
  onUsernameChange: (value: string) => void;
  onPasswordChange: (value: string) => void;
  onLogin: () => void;
};

export function LoginScreen({ username, password, error, onUsernameChange, onPasswordChange, onLogin }: Props) {
  return (
    <>
      <StatusBar style="dark" />
      <View style={styles.loginCard}>
        <Text style={styles.title}>登录好好记账</Text>
        <Text style={styles.muted}>当前仅开放固定账号登录，第三方登录后续接入。</Text>

        <View style={styles.row}>
          {(['账号', '微信', 'QQ', '手机', '邮箱'] as const).map((item, index) => (
            <TouchableOpacity
              key={item}
              disabled={index > 0}
              style={[styles.chip, index === 0 && styles.chipActive, index > 0 && styles.chipDisabled]}
            >
              <Text style={index === 0 ? styles.chipTextActive : styles.chipText}>{item}</Text>
            </TouchableOpacity>
          ))}
        </View>

        <TextInput
          style={styles.input}
          autoCapitalize="none"
          placeholder="用户名"
          value={username}
          onChangeText={onUsernameChange}
        />
        <TextInput
          style={styles.input}
          placeholder="密码"
          secureTextEntry
          value={password}
          onChangeText={onPasswordChange}
        />
        {!!error && <Text style={styles.error}>{error}</Text>}
        <TouchableOpacity style={styles.primaryBtn} onPress={onLogin}>
          <Text style={styles.primaryBtnText}>登录</Text>
        </TouchableOpacity>
      </View>
    </>
  );
}
