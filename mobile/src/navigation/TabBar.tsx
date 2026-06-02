import { ScrollView, Text, TouchableOpacity, View } from 'react-native';
import type { Tab } from '../shared/types/accounting';
import { styles } from '../theme/styles';
import { tabs } from './tabs';

type Props = {
  activeTab: Tab;
  onTabChange: (tab: Tab) => void;
};

export function TabBar({ activeTab, onTabChange }: Props) {
  return (
    <View style={styles.tabBar}>
      <ScrollView horizontal showsHorizontalScrollIndicator={false} contentContainerStyle={styles.tabBarContent}>
        {tabs.map(([key, label]) => (
          <TouchableOpacity key={key} style={styles.tabBtn} onPress={() => onTabChange(key)}>
            <Text style={activeTab === key ? styles.tabActive : styles.tabText}>{label}</Text>
          </TouchableOpacity>
        ))}
      </ScrollView>
    </View>
  );
}
