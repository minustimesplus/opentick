package org.opentick.android.widget

import android.content.Context
import androidx.glance.appwidget.GlanceAppWidget
import androidx.glance.appwidget.updateAll

/** Redraws the home screen widgets after their data changed. */
object Widgets {
    private val taskWidgets: List<GlanceAppWidget> get() = listOf(TasksWidget(), AddTaskWidget(), MatrixWidget(), CalendarWidget())
    private val habitWidgets: List<GlanceAppWidget> get() = listOf(TodayHabitsWidget(), WeeklyHabitsWidget())

    suspend fun updateTasks(context: Context) = taskWidgets.forEach { runCatching { it.updateAll(context) } }

    suspend fun updateHabits(context: Context) = habitWidgets.forEach { runCatching { it.updateAll(context) } }

    suspend fun updateAll(context: Context) { updateTasks(context); updateHabits(context) }

    /** The widget behind a provider class name, for its settings page. */
    fun forProvider(className: String?): GlanceAppWidget? = when (className) {
        TasksWidgetReceiver::class.java.name -> TasksWidget()
        AddTaskWidgetReceiver::class.java.name -> AddTaskWidget()
        MatrixWidgetReceiver::class.java.name -> MatrixWidget()
        CalendarWidgetReceiver::class.java.name -> CalendarWidget()
        TodayHabitsWidgetReceiver::class.java.name -> TodayHabitsWidget()
        WeeklyHabitsWidgetReceiver::class.java.name -> WeeklyHabitsWidget()
        else -> null
    }
}
